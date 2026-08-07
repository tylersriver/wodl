package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/application/command"
	"github.com/tyler/wodl/internal/application/common"
	"github.com/tyler/wodl/internal/application/query"
	"github.com/tyler/wodl/internal/application/services"
	"github.com/tyler/wodl/internal/infrastructure/middleware"
)

const (
	sessionDateLayout = "2006-01-02"
	monthParamLayout  = "2006-01"
)

type SessionHandler struct {
	sessionService *services.SessionService
	workoutService *services.WorkoutService
	liftService    *services.LiftService
	templates      *template.Template
	importEnabled  bool
}

func NewSessionHandler(sessionService *services.SessionService, workoutService *services.WorkoutService, liftService *services.LiftService, templates *template.Template, importEnabled bool) *SessionHandler {
	return &SessionHandler{
		sessionService: sessionService,
		workoutService: workoutService,
		liftService:    liftService,
		templates:      templates,
		importEnabled:  importEnabled,
	}
}

func (h *SessionHandler) List(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)

	// Week is the default: programming is planned a week at a time, so the
	// question this page usually answers is "what's on for the next few days"
	// rather than "everything I have ever planned".
	view := r.URL.Query().Get("view")
	switch view {
	case "calendar", "list":
		// accepted
	default:
		view = "week"
	}

	sessions, err := h.sessionService.GetSessionsByUser(&query.GetSessionsByUserQuery{UserId: userId})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	workouts, _ := h.workoutService.GetWorkoutsByUser(&query.GetWorkoutsByUserQuery{UserId: userId})

	loc := requestLocation(r)
	today := todayIn(loc)
	data := map[string]interface{}{
		"View":     view,
		"Sessions": sessions.Results,
		"Workouts": nil,
		"Today":    formatCivil(today),
		// The list has no notion of a day being looked at, so a new session
		// starts on today.
		"FormDate":      formatCivil(today),
		"ImportEnabled": h.importEnabled,
	}
	if workouts != nil {
		data["Workouts"] = workouts.Results
	}

	switch view {
	case "calendar":
		month, err := parseMonthParam(r.URL.Query().Get("month"), today)
		if err != nil {
			http.Error(w, "invalid month", http.StatusBadRequest)
			return
		}
		cal, err := h.buildCalendar(userId, month, today, loc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data["Calendar"] = cal
	case "week":
		weekStart, err := parseWeekParam(r.URL.Query().Get("week"), today)
		if err != nil {
			http.Error(w, "invalid week", http.StatusBadRequest)
			return
		}
		wk, err := h.buildWeek(userId, weekStart, today, loc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data["Week"] = wk
	}

	h.templates.ExecuteTemplate(w, "sessions.html", data)
}

func (h *SessionHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	userId := middleware.GetUserID(r)

	today := todayIn(requestLocation(r))
	cmd := &command.CreateSessionCommand{
		UserId:     userId,
		Name:       r.FormValue("name"),
		Warmup:     r.FormValue("warmup"),
		Date:       parseSessionDate(r.FormValue("date"), today),
		WorkoutIds: parseWorkoutIds(r),
	}
	if v := r.FormValue("total_time_minutes"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cmd.TotalTimeMinutes = &n
		}
	}
	if cmd.Name == "" {
		cmd.Name = defaultSessionName(cmd.Date)
	}

	created, err := h.sessionService.CreateSession(cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Land on the day the session was assigned to, which is usually the plan
	// the user is about to follow.
	http.Redirect(w, r, sessionRedirectTarget(created.Result.Date, today), http.StatusSeeOther)
}

func (h *SessionHandler) Detail(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	result, err := h.sessionService.GetSessionById(&query.GetSessionByIdQuery{Id: id, UserId: userId})
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if result.Session != nil {
		for _, wr := range result.Session.Workouts {
			enrichLiftingWorkout(wr, h.liftService, userId)
		}
	}

	workouts, _ := h.workoutService.GetWorkoutsByUser(&query.GetWorkoutsByUserQuery{UserId: userId})

	dateStr := ""
	orderedIds := make([]string, 0)
	if result.Session != nil {
		dateStr = formatCivil(result.Session.Date)
		for _, wr := range result.Session.Workouts {
			orderedIds = append(orderedIds, wr.Id.String())
		}
	}

	data := map[string]interface{}{
		"Session":         result.Session,
		"Workouts":        nil,
		"DateStr":         dateStr,
		"Today":           formatCivil(todayIn(requestLocation(r))),
		"WorkoutOrderCSV": strings.Join(orderedIds, ","),
	}
	if workouts != nil {
		data["Workouts"] = workouts.Results
	}
	h.templates.ExecuteTemplate(w, "session_detail.html", data)
}

func (h *SessionHandler) Update(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	r.ParseForm()
	cmd := &command.UpdateSessionCommand{
		Id:         id,
		UserId:     userId,
		Name:       r.FormValue("name"),
		Warmup:     r.FormValue("warmup"),
		Date:       parseSessionDate(r.FormValue("date"), todayIn(requestLocation(r))),
		WorkoutIds: parseWorkoutIds(r),
	}
	if v := r.FormValue("total_time_minutes"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cmd.TotalTimeMinutes = &n
		}
	}
	if cmd.Name == "" {
		cmd.Name = defaultSessionName(cmd.Date)
	}

	if err := h.sessionService.UpdateSession(cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/sessions/%s", id), http.StatusSeeOther)
}

func (h *SessionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.sessionService.DeleteSession(&command.DeleteSessionCommand{Id: id, UserId: userId}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/sessions")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/sessions", http.StatusSeeOther)
}

// sessionRedirectTarget sends the user to the Today page when the session they
// just saved is for today, and to the sessions list otherwise.
func sessionRedirectTarget(date, today time.Time) string {
	if civilDate(date).Equal(today) {
		return "/"
	}
	return "/sessions"
}

// defaultSessionName names an unnamed session after the day it is assigned to,
// so the user never has to invent a title for "the workout on Tuesday".
func defaultSessionName(date time.Time) string {
	return date.Format("Mon, Jan 2")
}

// parseMonthParam returns the first-of-month time for a YYYY-MM input. Empty
// string yields the month the user is currently in.
func parseMonthParam(v string, today time.Time) (time.Time, error) {
	if v == "" {
		return time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC), nil
	}
	t, err := time.Parse(monthParamLayout, v)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

// CalendarDay is one cell in the month grid. If InMonth is false the day belongs
// to the preceding or following month and is rendered greyed out.
type CalendarDay struct {
	Date    time.Time
	InMonth bool
	IsToday bool
	// Logged marks a day the user trained: anything at all was recorded on it,
	// whether or not the day had a plan or the plan matched.
	Logged bool
	// PlanLogged narrows that to a day whose own plan was at least started:
	// something one of its sessions prescribes was recorded on it. A session is
	// only ever a plan, so this is the nearest thing to "done" the model has —
	// see loggedIndex. It implies Logged.
	PlanLogged bool
	Sessions   []*common.SessionResult
}

// CalendarMonth holds everything the template needs to render a month view.
type CalendarMonth struct {
	Month      time.Time
	MonthLabel string
	PrevParam  string
	NextParam  string
	CurrParam  string
	Weeks      [][]CalendarDay
}

func (h *SessionHandler) buildCalendar(userId uuid.UUID, monthStart, today time.Time, loc *time.Location) (*CalendarMonth, error) {
	// Range spans the 6-row grid we always render, so leading/trailing days
	// from neighbouring months are included if they carry sessions.
	gridStart := monthStart.AddDate(0, 0, -int(monthStart.Weekday()))
	gridEnd := gridStart.AddDate(0, 0, 42)

	byDay, err := h.sessionsByDay(userId, gridStart, gridEnd)
	if err != nil {
		return nil, err
	}

	logged, err := h.buildLoggedIndex(userId, gridStart, gridEnd, loc)
	if err != nil {
		return nil, err
	}

	weeks := make([][]CalendarDay, 6)
	for w := 0; w < 6; w++ {
		weeks[w] = make([]CalendarDay, 7)
		for d := 0; d < 7; d++ {
			day := gridStart.AddDate(0, 0, w*7+d)
			key := formatCivil(day)
			weeks[w][d] = CalendarDay{
				Date:       day,
				InMonth:    day.Month() == monthStart.Month() && day.Year() == monthStart.Year(),
				IsToday:    day.Equal(today),
				Logged:     logged.loggedOn(key),
				PlanLogged: logged.anyDone(byDay[key]),
				Sessions:   byDay[key],
			}
		}
	}

	return &CalendarMonth{
		Month:      monthStart,
		MonthLabel: monthStart.Format("January 2006"),
		PrevParam:  monthStart.AddDate(0, -1, 0).UTC().Format(monthParamLayout),
		NextParam:  monthStart.AddDate(0, 1, 0).UTC().Format(monthParamLayout),
		CurrParam:  monthStart.UTC().Format(monthParamLayout),
		Weeks:      weeks,
	}, nil
}

// parseWeekParam returns the Sunday-start of the week containing the given
// YYYY-MM-DD date (or today if empty).
func parseWeekParam(v string, today time.Time) (time.Time, error) {
	day := today
	if v != "" {
		t, err := parseCivilDate(v)
		if err != nil {
			return time.Time{}, err
		}
		day = t
	}
	return day.AddDate(0, 0, -int(day.Weekday())), nil
}

// CalendarWeek holds data for rendering a single-week calendar view.
type CalendarWeek struct {
	Start     time.Time
	End       time.Time
	Label     string
	PrevParam string
	NextParam string
	CurrParam string
	Days      []CalendarDay
}

func (h *SessionHandler) buildWeek(userId uuid.UUID, weekStart, today time.Time, loc *time.Location) (*CalendarWeek, error) {
	weekEnd := weekStart.AddDate(0, 0, 7)

	byDay, err := h.sessionsByDay(userId, weekStart, weekEnd)
	if err != nil {
		return nil, err
	}

	logged, err := h.buildLoggedIndex(userId, weekStart, weekEnd, loc)
	if err != nil {
		return nil, err
	}

	days := make([]CalendarDay, 7)
	for i := 0; i < 7; i++ {
		day := weekStart.AddDate(0, 0, i)
		key := formatCivil(day)
		days[i] = CalendarDay{
			Date:       day,
			InMonth:    true,
			IsToday:    day.Equal(today),
			Logged:     logged.loggedOn(key),
			PlanLogged: logged.anyDone(byDay[key]),
			Sessions:   byDay[key],
		}
	}

	lastDay := weekStart.AddDate(0, 0, 6)
	var label string
	if weekStart.Month() == lastDay.Month() {
		label = fmt.Sprintf("%s %d – %d, %d",
			weekStart.Month().String(), weekStart.Day(), lastDay.Day(), weekStart.Year())
	} else {
		label = fmt.Sprintf("%s %d – %s %d, %d",
			weekStart.Month().String(), weekStart.Day(),
			lastDay.Month().String(), lastDay.Day(), lastDay.Year())
	}

	return &CalendarWeek{
		Start:     weekStart,
		End:       lastDay,
		Label:     label,
		PrevParam: formatCivil(weekStart.AddDate(0, 0, -7)),
		NextParam: formatCivil(weekStart.AddDate(0, 0, 7)),
		CurrParam: formatCivil(weekStart),
		Days:      days,
	}, nil
}

// loggedIndex answers two questions about a day, and the difference between
// them is the whole point: "did you train?" and "did you do the plan?".
//
// The first is just presence — anything logged on the day, plan or not, counts.
// A day you trained is a day you trained even if you never wrote a session for
// it, or wrote one and did something else. That is `loggedOn`.
//
// The second is narrower, and nothing records it directly: a session is the
// plan, and results are recorded against the individual lift or workout. So it
// is answered by coincidence in time — a day's plan counts as started when
// something one of its sessions prescribes was logged on that same day. Matching
// on the day as well as the workout is what keeps a repeated benchmark from
// marking every session it has ever appeared in. That is `done`.
//
// Note that both are keyed by the day the result was *logged on*, not the day
// some session prescribed it: logging Saturday's workout on Sunday marks Sunday,
// because Sunday is when you were in the gym.
//
// Lifting steps are held apart because sets are logged against the lift itself,
// not against the workout that prescribed them, so those never produce a
// workout_result to find.
type loggedIndex struct {
	days     map[string]bool
	workouts map[string]map[uuid.UUID]bool
	lifts    map[string]map[uuid.UUID]bool
}

// loggedOn reports whether the user recorded anything at all on the given civil
// day, regardless of what — if anything — was planned for it.
func (l *loggedIndex) loggedOn(day string) bool {
	if l == nil {
		return false
	}
	return l.days[day]
}

// done reports whether anything in the session was logged on the session's own
// day.
func (l *loggedIndex) done(s *common.SessionResult) bool {
	if l == nil || s == nil {
		return false
	}
	day := formatCivil(s.Date)
	for _, w := range s.Workouts {
		if l.workouts[day][w.Id] {
			return true
		}
		if w.LiftId != nil && l.lifts[day][*w.LiftId] {
			return true
		}
	}
	return false
}

// anyDone reports whether any of a day's sessions was started.
func (l *loggedIndex) anyDone(sessions []*common.SessionResult) bool {
	for _, s := range sessions {
		if l.done(s) {
			return true
		}
	}
	return false
}

// buildLoggedIndex reads everything the user logged over the civil days
// [start, end) — measured in their own zone, since that is where the days the
// grid draws are — and indexes it by the day it was logged on.
func (h *SessionHandler) buildLoggedIndex(userId uuid.UUID, start, end time.Time, loc *time.Location) (*loggedIndex, error) {
	from, to := startOfDayIn(start, loc), startOfDayIn(end, loc)

	idx := &loggedIndex{
		days:     map[string]bool{},
		workouts: map[string]map[uuid.UUID]bool{},
		lifts:    map[string]map[uuid.UUID]bool{},
	}

	results, err := h.workoutService.GetWorkoutResultsInRange(&query.GetWorkoutResultsInRangeQuery{
		UserId: userId,
		Start:  from,
		End:    to,
	})
	if err != nil {
		return nil, err
	}
	for _, r := range results.Results {
		day := formatCivil(dayIn(r.LoggedAt, loc))
		idx.days[day] = true
		mark(idx.workouts, day, r.WorkoutId)
	}

	logs, err := h.liftService.GetLiftLogsInRange(&query.GetLiftLogsInRangeQuery{
		UserId: userId,
		Start:  from,
		End:    to,
	})
	if err != nil {
		return nil, err
	}
	for _, l := range logs.Results {
		day := formatCivil(dayIn(l.LoggedAt, loc))
		idx.days[day] = true
		mark(idx.lifts, day, l.LiftId)
	}

	return idx, nil
}

func mark(m map[string]map[uuid.UUID]bool, day string, id uuid.UUID) {
	if m[day] == nil {
		m[day] = map[uuid.UUID]bool{}
	}
	m[day][id] = true
}

// sessionsByDay buckets the sessions in [start, end) by their calendar day.
func (h *SessionHandler) sessionsByDay(userId uuid.UUID, start, end time.Time) (map[string][]*common.SessionResult, error) {
	sessions, err := h.sessionService.GetSessionsInRange(&query.GetSessionsInRangeQuery{
		UserId: userId,
		Start:  start,
		End:    end,
	})
	if err != nil {
		return nil, err
	}

	byDay := map[string][]*common.SessionResult{}
	for _, s := range sessions.Results {
		key := formatCivil(s.Date)
		byDay[key] = append(byDay[key], s)
	}
	return byDay, nil
}

// parseSessionDate parses a YYYY-MM-DD string from a form input, falling back to
// today when it is missing or malformed. A session always belongs to a day.
func parseSessionDate(v string, today time.Time) time.Time {
	if v != "" {
		if t, err := parseCivilDate(v); err == nil {
			return t
		}
	}
	return today
}

// parseWorkoutIds collects all workout_ids[] form values (in submission order)
// and returns the ones that parse as valid UUIDs.
func parseWorkoutIds(r *http.Request) []uuid.UUID {
	raw := r.Form["workout_ids"]
	ids := make([]uuid.UUID, 0, len(raw))
	for _, s := range raw {
		if s == "" {
			continue
		}
		if id, err := uuid.Parse(s); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

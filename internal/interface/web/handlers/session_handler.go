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
}

func NewSessionHandler(sessionService *services.SessionService, workoutService *services.WorkoutService, liftService *services.LiftService, templates *template.Template) *SessionHandler {
	return &SessionHandler{
		sessionService: sessionService,
		workoutService: workoutService,
		liftService:    liftService,
		templates:      templates,
	}
}

func (h *SessionHandler) List(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)

	view := r.URL.Query().Get("view")
	switch view {
	case "calendar", "week":
		// accepted
	default:
		view = "list"
	}

	sessions, err := h.sessionService.GetSessionsByUser(&query.GetSessionsByUserQuery{UserId: userId})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	workouts, _ := h.workoutService.GetWorkoutsByUser(&query.GetWorkoutsByUserQuery{UserId: userId})

	data := map[string]interface{}{
		"View":     view,
		"Sessions": sessions.Results,
		"Workouts": nil,
		"Today":    time.Now().Format(sessionDateLayout),
	}
	if workouts != nil {
		data["Workouts"] = workouts.Results
	}

	switch view {
	case "calendar":
		month, err := parseMonthParam(r.URL.Query().Get("month"))
		if err != nil {
			http.Error(w, "invalid month", http.StatusBadRequest)
			return
		}
		cal, err := h.buildCalendar(userId, month)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data["Calendar"] = cal
	case "week":
		weekStart, err := parseWeekParam(r.URL.Query().Get("week"))
		if err != nil {
			http.Error(w, "invalid week", http.StatusBadRequest)
			return
		}
		wk, err := h.buildWeek(userId, weekStart)
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

	cmd := &command.CreateSessionCommand{
		UserId:     userId,
		Name:       r.FormValue("name"),
		Warmup:     r.FormValue("warmup"),
		Date:       parseSessionDate(r.FormValue("date")),
		WorkoutIds: parseWorkoutIds(r),
	}
	if v := r.FormValue("total_time_minutes"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cmd.TotalTimeMinutes = &n
		}
	}

	_, err := h.sessionService.CreateSession(cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/sessions", http.StatusSeeOther)
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
		if result.Session.Date != nil {
			dateStr = result.Session.Date.Format(sessionDateLayout)
		}
		for _, wr := range result.Session.Workouts {
			orderedIds = append(orderedIds, wr.Id.String())
		}
	}

	data := map[string]interface{}{
		"Session":         result.Session,
		"Workouts":        nil,
		"DateStr":         dateStr,
		"Today":           time.Now().Format(sessionDateLayout),
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
		Date:       parseSessionDate(r.FormValue("date")),
		WorkoutIds: parseWorkoutIds(r),
	}
	if v := r.FormValue("total_time_minutes"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cmd.TotalTimeMinutes = &n
		}
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

func (h *SessionHandler) CreateLog(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	sessionId, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	r.ParseForm()
	performed := parseSessionDate(r.FormValue("performed_at"))
	if performed == nil {
		now := time.Now()
		performed = &now
	}

	cmd := &command.CreateSessionLogCommand{
		UserId:      userId,
		SessionId:   sessionId,
		PerformedAt: *performed,
		Notes:       r.FormValue("notes"),
	}

	if _, err := h.sessionService.CreateSessionLog(cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/sessions/%s", sessionId), http.StatusSeeOther)
}

func (h *SessionHandler) DeleteLog(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	logId, err := uuid.Parse(chi.URLParam(r, "logId"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	sessionId := chi.URLParam(r, "id")

	if err := h.sessionService.DeleteSessionLog(&command.DeleteSessionLogCommand{Id: logId, UserId: userId}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", fmt.Sprintf("/sessions/%s", sessionId))
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/sessions/%s", sessionId), http.StatusSeeOther)
}

// parseMonthParam returns the first-of-month time for a YYYY-MM input. Empty
// string yields the current month.
func parseMonthParam(v string) (time.Time, error) {
	if v == "" {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local), nil
	}
	t, err := time.ParseInLocation(monthParamLayout, v, time.Local)
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
	Logs    []*common.SessionLogResult
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

func (h *SessionHandler) buildCalendar(userId uuid.UUID, monthStart time.Time) (*CalendarMonth, error) {
	// Range spans the 6-row grid we always render, so leading/trailing days
	// from neighbouring months are included if they carry logs.
	gridStart := monthStart.AddDate(0, 0, -int(monthStart.Weekday()))
	gridEnd := gridStart.AddDate(0, 0, 42)

	logs, err := h.sessionService.GetSessionLogsInRange(&query.GetSessionLogsInRangeQuery{
		UserId: userId,
		Start:  gridStart,
		End:    gridEnd,
	})
	if err != nil {
		return nil, err
	}

	byDay := map[string][]*common.SessionLogResult{}
	for _, l := range logs.Results {
		key := l.PerformedAt.In(time.Local).Format(sessionDateLayout)
		byDay[key] = append(byDay[key], l)
	}

	today := time.Now().Format(sessionDateLayout)
	weeks := make([][]CalendarDay, 6)
	for w := 0; w < 6; w++ {
		weeks[w] = make([]CalendarDay, 7)
		for d := 0; d < 7; d++ {
			day := gridStart.AddDate(0, 0, w*7+d)
			key := day.Format(sessionDateLayout)
			weeks[w][d] = CalendarDay{
				Date:    day,
				InMonth: day.Month() == monthStart.Month() && day.Year() == monthStart.Year(),
				IsToday: key == today,
				Logs:    byDay[key],
			}
		}
	}

	return &CalendarMonth{
		Month:      monthStart,
		MonthLabel: monthStart.Format("January 2006"),
		PrevParam:  monthStart.AddDate(0, -1, 0).Format(monthParamLayout),
		NextParam:  monthStart.AddDate(0, 1, 0).Format(monthParamLayout),
		CurrParam:  monthStart.Format(monthParamLayout),
		Weeks:      weeks,
	}, nil
}

// parseWeekParam returns the Sunday-start of the week containing the given
// YYYY-MM-DD date (or today if empty).
func parseWeekParam(v string) (time.Time, error) {
	var anchor time.Time
	if v == "" {
		anchor = time.Now()
	} else {
		t, err := time.ParseInLocation(sessionDateLayout, v, time.Local)
		if err != nil {
			return time.Time{}, err
		}
		anchor = t
	}
	day := time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, time.Local)
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

func (h *SessionHandler) buildWeek(userId uuid.UUID, weekStart time.Time) (*CalendarWeek, error) {
	weekEnd := weekStart.AddDate(0, 0, 7)

	logs, err := h.sessionService.GetSessionLogsInRange(&query.GetSessionLogsInRangeQuery{
		UserId: userId,
		Start:  weekStart,
		End:    weekEnd,
	})
	if err != nil {
		return nil, err
	}

	byDay := map[string][]*common.SessionLogResult{}
	for _, l := range logs.Results {
		key := l.PerformedAt.In(time.Local).Format(sessionDateLayout)
		byDay[key] = append(byDay[key], l)
	}

	today := time.Now().Format(sessionDateLayout)
	days := make([]CalendarDay, 7)
	for i := 0; i < 7; i++ {
		day := weekStart.AddDate(0, 0, i)
		key := day.Format(sessionDateLayout)
		days[i] = CalendarDay{
			Date:    day,
			InMonth: true,
			IsToday: key == today,
			Logs:    byDay[key],
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
		PrevParam: weekStart.AddDate(0, 0, -7).Format(sessionDateLayout),
		NextParam: weekStart.AddDate(0, 0, 7).Format(sessionDateLayout),
		CurrParam: weekStart.Format(sessionDateLayout),
		Days:      days,
	}, nil
}

// parseSessionDate parses a YYYY-MM-DD string from a form input; returns nil
// when empty or malformed.
func parseSessionDate(v string) *time.Time {
	if v == "" {
		return nil
	}
	t, err := time.ParseInLocation(sessionDateLayout, v, time.Local)
	if err != nil {
		return nil
	}
	return &t
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

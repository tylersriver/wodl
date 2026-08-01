package handlers

import (
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/tyler/wodl/internal/application/common"
	"github.com/tyler/wodl/internal/application/query"
	"github.com/tyler/wodl/internal/application/services"
	"github.com/tyler/wodl/internal/domain/entities"
	"github.com/tyler/wodl/internal/infrastructure/middleware"
)

type DashboardHandler struct {
	liftService    *services.LiftService
	workoutService *services.WorkoutService
	sessionService *services.SessionService
	templates      *template.Template
	// importEnabled gates the "import from a photo" entry point, which only
	// works when an API key is configured.
	importEnabled bool
}

func NewDashboardHandler(liftService *services.LiftService, workoutService *services.WorkoutService, sessionService *services.SessionService, templates *template.Template, importEnabled bool) *DashboardHandler {
	return &DashboardHandler{
		liftService:    liftService,
		workoutService: workoutService,
		sessionService: sessionService,
		templates:      templates,
		importEnabled:  importEnabled,
	}
}

// Today renders the session assigned to today — the workout of the day. It is
// the landing page, so the first thing the user sees is what they are meant to
// be doing rather than a search box.
func (h *DashboardHandler) Today(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)

	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	todays, err := h.sessionService.GetSessionsInRange(&query.GetSessionsInRangeQuery{
		UserId: userId,
		Start:  dayStart,
		End:    dayStart.AddDate(0, 0, 1),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Lifting workouts carry a percentage table off their linked lift, which is
	// the whole point of seeing them on the day.
	var sessions []*common.SessionResult
	if todays != nil {
		sessions = todays.Results
		for _, s := range sessions {
			for _, wr := range s.Workouts {
				enrichLiftingWorkout(wr, h.liftService, userId)
			}
		}
	}

	workouts, _ := h.workoutService.GetWorkoutsByUser(&query.GetWorkoutsByUserQuery{UserId: userId})

	data := map[string]interface{}{
		"Sessions":  sessions,
		"Workouts":  nil,
		"TodayText": now.Format("Monday, January 2"),
		"Today":     now.Format(sessionDateLayout),
		// Lets the template suppress an auto-generated session name, which
		// would otherwise just repeat the date already in the page heading.
		"DefaultName":   defaultSessionName(now),
		"ImportEnabled": h.importEnabled,
	}
	if workouts != nil {
		data["Workouts"] = workouts.Results
	}

	h.templates.ExecuteTemplate(w, "today.html", data)
}

// Results renders lifts and workouts in one filterable list. They were
// previously two separate tabs, but from the user's point of view both are just
// "things I have results for".
func (h *DashboardHandler) Results(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)

	kind := r.URL.Query().Get("kind")
	switch kind {
	case "lifts", "workouts":
		// accepted
	default:
		kind = "all"
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	var lifts []*common.LiftResult
	var workouts []*common.WorkoutResult

	if kind != "workouts" {
		if res, err := h.liftService.GetLiftsByUser(&query.GetLiftsByUserQuery{UserId: userId}); err == nil && res != nil {
			for _, l := range res.Results {
				if matchesQuery(q, l.Name, l.Category) {
					lifts = append(lifts, l)
				}
			}
		}
	}

	// Lifting workouts are auto-titled from a lift plus a date, so they'd bury
	// the real workouts in noise; the lift itself already appears in this list.
	// Kept reachable via ?include_lifting=1, as on the old workouts page.
	includeLifting := r.URL.Query().Get("include_lifting") == "1"

	if kind != "lifts" {
		if res, err := h.workoutService.GetWorkoutsByUser(&query.GetWorkoutsByUserQuery{UserId: userId}); err == nil && res != nil {
			for _, wo := range res.Results {
				if !includeLifting && wo.Type == string(entities.WorkoutTypeLifting) {
					continue
				}
				if matchesQuery(q, wo.Name, wo.Type, wo.Description) {
					workouts = append(workouts, wo)
				}
			}
		}
	}

	data := map[string]interface{}{
		"Lifts":          lifts,
		"Workouts":       workouts,
		"Kind":           kind,
		"Query":          r.URL.Query().Get("q"),
		"Filtered":       q != "" || kind != "all",
		"IncludeLifting": includeLifting,
		"Categories":     entities.ValidLiftCategories(),
		"WorkoutTypes":   entities.ValidWorkoutTypes(),
		"Today":          time.Now().Format(sessionDateLayout),
	}

	// The filter posts back over htmx, so re-render just the list when asked.
	if r.Header.Get("HX-Request") == "true" {
		h.templates.ExecuteTemplate(w, "results_list", data)
		return
	}
	h.templates.ExecuteTemplate(w, "results.html", data)
}

// matchesQuery reports whether any of the given fields contains q. An empty
// query matches everything.
func matchesQuery(q string, fields ...string) bool {
	if q == "" {
		return true
	}
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

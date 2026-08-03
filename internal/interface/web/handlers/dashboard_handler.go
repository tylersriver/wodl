package handlers

import (
	"html/template"
	"net/http"
	"strings"

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

// Today renders the session assigned to one day — the workout of the day. It is
// the landing page, so the first thing the user sees is what they are meant to
// be doing rather than a search box.
//
// The day is addressable with ?date=YYYY-MM-DD, which is what makes stepping
// between days possible at all: the arrows and the swipe gesture are both just
// links to the neighbouring dates, so the feature works without JavaScript and
// every day can be bookmarked. An unparseable date falls back to today rather
// than erroring, since it only ever arrives from a hand-edited URL.
//
// Which day is "today" is the reader's question, not the server's, so it is
// resolved in the zone their browser reported — see requestLocation.
func (h *DashboardHandler) Today(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)

	today := todayIn(requestLocation(r))
	day := today
	if v := strings.TrimSpace(r.URL.Query().Get("date")); v != "" {
		if parsed, err := parseCivilDate(v); err == nil {
			day = parsed
		}
	}

	todays, err := h.sessionService.GetSessionsInRange(&query.GetSessionsInRangeQuery{
		UserId: userId,
		Start:  day,
		End:    day.AddDate(0, 0, 1),
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
		"TodayText": day.Format("Monday, January 2"),
		// Heads the page when the day isn't today; the full date sits right
		// above it, so the weekday alone is unambiguous enough.
		"WeekdayText": day.Format("Monday"),
		// Neighbouring days for the arrows and the swipe gesture. Both directions
		// are always offered: an empty day is a normal thing to land on, and is
		// where you plan one.
		"PrevDate": formatCivil(day.AddDate(0, 0, -1)),
		"NextDate": formatCivil(day.AddDate(0, 0, 1)),
		"IsToday":  day.Equal(today),
		// Planning from this page should plan the day being looked at, not today.
		"FormDate": formatCivil(day),
		// Lets the template suppress an auto-generated session name, which
		// would otherwise just repeat the date already in the page heading.
		"DefaultName":   defaultSessionName(day),
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
		"Today":          formatCivil(todayIn(requestLocation(r))),
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

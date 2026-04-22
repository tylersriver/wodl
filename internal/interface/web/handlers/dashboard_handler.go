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
}

func NewDashboardHandler(liftService *services.LiftService, workoutService *services.WorkoutService, sessionService *services.SessionService, templates *template.Template) *DashboardHandler {
	return &DashboardHandler{
		liftService:    liftService,
		workoutService: workoutService,
		sessionService: sessionService,
		templates:      templates,
	}
}

func (h *DashboardHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)

	recentLogs, _ := h.liftService.GetRecentLiftLogs(&query.GetRecentLiftLogsQuery{
		UserId: userId, Limit: 5,
	})

	recentResults, _ := h.workoutService.GetRecentWorkoutResults(&query.GetRecentWorkoutResultsQuery{
		UserId: userId, Limit: 5,
	})

	lifts, _ := h.liftService.GetLiftsByUser(&query.GetLiftsByUserQuery{UserId: userId})
	workouts, _ := h.workoutService.GetWorkoutsByUser(&query.GetWorkoutsByUserQuery{UserId: userId})
	sessions, _ := h.sessionService.GetSessionsByUser(&query.GetSessionsByUserQuery{UserId: userId})

	data := map[string]interface{}{
		"RecentLogs":    nil,
		"RecentResults": nil,
		"Lifts":         nil,
		"Workouts":      nil,
		"Sessions":      nil,
		"Categories":    entities.ValidLiftCategories(),
		"WorkoutTypes":  entities.ValidWorkoutTypes(),
	}
	if recentLogs != nil {
		data["RecentLogs"] = recentLogs.Results
	}
	if recentResults != nil {
		data["RecentResults"] = recentResults.Results
	}
	if lifts != nil {
		data["Lifts"] = lifts.Results
	}
	if workouts != nil {
		data["Workouts"] = workouts.Results
	}
	if sessions != nil {
		data["Sessions"] = sessions.Results
	}

	h.templates.ExecuteTemplate(w, "dashboard.html", data)
}

func (h *DashboardHandler) Search(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	if q == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var matchedLifts []*common.LiftResult
	var matchedWorkouts []*common.WorkoutResult
	var matchedSessions []*common.SessionResult

	if lifts, err := h.liftService.GetLiftsByUser(&query.GetLiftsByUserQuery{UserId: userId}); err == nil && lifts != nil {
		for _, l := range lifts.Results {
			if strings.Contains(strings.ToLower(l.Name), q) || strings.Contains(strings.ToLower(l.Category), q) {
				matchedLifts = append(matchedLifts, l)
			}
		}
	}

	if workouts, err := h.workoutService.GetWorkoutsByUser(&query.GetWorkoutsByUserQuery{UserId: userId}); err == nil && workouts != nil {
		for _, w := range workouts.Results {
			if strings.Contains(strings.ToLower(w.Name), q) || strings.Contains(strings.ToLower(w.Type), q) {
				matchedWorkouts = append(matchedWorkouts, w)
			}
		}
	}

	if sessions, err := h.sessionService.GetSessionsByUser(&query.GetSessionsByUserQuery{UserId: userId}); err == nil && sessions != nil {
		for _, s := range sessions.Results {
			if strings.Contains(strings.ToLower(s.Name), q) || strings.Contains(strings.ToLower(s.Warmup), q) {
				matchedSessions = append(matchedSessions, s)
			}
		}
	}

	data := map[string]interface{}{
		"Lifts":    matchedLifts,
		"Workouts": matchedWorkouts,
		"Sessions": matchedSessions,
	}
	h.templates.ExecuteTemplate(w, "search_results.html", data)
}

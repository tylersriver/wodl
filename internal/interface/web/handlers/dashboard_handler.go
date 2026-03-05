package handlers

import (
	"html/template"
	"net/http"

	"github.com/tyler/wodl/internal/application/query"
	"github.com/tyler/wodl/internal/application/services"
	"github.com/tyler/wodl/internal/infrastructure/middleware"
)

type DashboardHandler struct {
	liftService    *services.LiftService
	workoutService *services.WorkoutService
	templates      *template.Template
}

func NewDashboardHandler(liftService *services.LiftService, workoutService *services.WorkoutService, templates *template.Template) *DashboardHandler {
	return &DashboardHandler{liftService: liftService, workoutService: workoutService, templates: templates}
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

	data := map[string]interface{}{
		"RecentLogs":    nil,
		"RecentResults": nil,
		"Lifts":         nil,
		"Workouts":      nil,
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

	h.templates.ExecuteTemplate(w, "dashboard.html", data)
}

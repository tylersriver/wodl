package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/application/command"
	"github.com/tyler/wodl/internal/application/query"
	"github.com/tyler/wodl/internal/application/services"
	"github.com/tyler/wodl/internal/domain/entities"
	"github.com/tyler/wodl/internal/infrastructure/middleware"
)

type WorkoutHandler struct {
	workoutService *services.WorkoutService
	templates      *template.Template
}

func NewWorkoutHandler(workoutService *services.WorkoutService, templates *template.Template) *WorkoutHandler {
	return &WorkoutHandler{workoutService: workoutService, templates: templates}
}

func (h *WorkoutHandler) List(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	result, err := h.workoutService.GetWorkoutsByUser(&query.GetWorkoutsByUserQuery{UserId: userId})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.templates.ExecuteTemplate(w, "workouts.html", map[string]interface{}{
		"Workouts":     result.Results,
		"WorkoutTypes": entities.ValidWorkoutTypes(),
		"ScoreTypes":   entities.ValidScoreTypes(),
	})
}

func (h *WorkoutHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	userId := middleware.GetUserID(r)

	cmd := &command.CreateWorkoutCommand{
		UserId:      userId,
		Name:        r.FormValue("name"),
		Type:        r.FormValue("type"),
		Description: r.FormValue("description"),
	}

	if v := r.FormValue("time_cap"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cmd.TimeCap = &n
		}
	}
	if v := r.FormValue("rounds"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cmd.Rounds = &n
		}
	}
	if v := r.FormValue("interval_seconds"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cmd.IntervalSeconds = &n
		}
	}

	_, err := h.workoutService.CreateWorkout(cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/workouts", http.StatusSeeOther)
}

func (h *WorkoutHandler) Detail(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	result, err := h.workoutService.GetWorkoutById(&query.GetWorkoutByIdQuery{Id: id, UserId: userId})
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.templates.ExecuteTemplate(w, "workout_detail.html", map[string]interface{}{
		"Workout":      result.Workout,
		"Results":      result.Results,
		"WorkoutTypes": entities.ValidWorkoutTypes(),
		"ScoreTypes":   entities.ValidScoreTypes(),
	})
}

func (h *WorkoutHandler) Update(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	r.ParseForm()
	cmd := &command.UpdateWorkoutCommand{
		Id:          id,
		UserId:      userId,
		Name:        r.FormValue("name"),
		Type:        r.FormValue("type"),
		Description: r.FormValue("description"),
	}

	if v := r.FormValue("time_cap"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cmd.TimeCap = &n
		}
	}
	if v := r.FormValue("rounds"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cmd.Rounds = &n
		}
	}
	if v := r.FormValue("interval_seconds"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cmd.IntervalSeconds = &n
		}
	}

	if err := h.workoutService.UpdateWorkout(cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/workouts/%s", id), http.StatusSeeOther)
}

func (h *WorkoutHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.workoutService.DeleteWorkout(&command.DeleteWorkoutCommand{Id: id, UserId: userId}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/workouts")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/workouts", http.StatusSeeOther)
}

func (h *WorkoutHandler) CreateResult(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	workoutId, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	r.ParseForm()
	cmd := &command.CreateWorkoutResultCommand{
		UserId:    userId,
		WorkoutId: workoutId,
		Score:     r.FormValue("score"),
		ScoreType: r.FormValue("score_type"),
		Rx:        r.FormValue("rx") == "on",
		Notes:     r.FormValue("notes"),
	}

	_, err = h.workoutService.CreateWorkoutResult(cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/workouts/%s", workoutId), http.StatusSeeOther)
}

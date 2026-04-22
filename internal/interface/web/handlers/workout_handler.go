package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/application/command"
	"github.com/tyler/wodl/internal/application/common"
	"github.com/tyler/wodl/internal/application/query"
	"github.com/tyler/wodl/internal/application/services"
	"github.com/tyler/wodl/internal/domain/entities"
	"github.com/tyler/wodl/internal/infrastructure/middleware"
)

type WorkoutHandler struct {
	workoutService *services.WorkoutService
	liftService    *services.LiftService
	templates      *template.Template
}

func NewWorkoutHandler(workoutService *services.WorkoutService, liftService *services.LiftService, templates *template.Template) *WorkoutHandler {
	return &WorkoutHandler{workoutService: workoutService, liftService: liftService, templates: templates}
}

func (h *WorkoutHandler) List(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	result, err := h.workoutService.GetWorkoutsByUser(&query.GetWorkoutsByUserQuery{UserId: userId})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	lifts, _ := h.liftService.GetLiftsByUser(&query.GetLiftsByUserQuery{UserId: userId})

	// Lifting workouts are excluded from the list by default since they're
	// driven by their linked Lift's 1RM table rather than a standalone
	// prescription; users can opt in with ?include_lifting=1.
	includeLifting := r.URL.Query().Get("include_lifting") == "1"
	visible := result.Results
	if !includeLifting {
		filtered := visible[:0:0]
		for _, wr := range visible {
			if wr.Type != string(entities.WorkoutTypeLifting) {
				filtered = append(filtered, wr)
			}
		}
		visible = filtered
	}

	data := map[string]interface{}{
		"Workouts":       visible,
		"WorkoutTypes":   entities.ValidWorkoutTypes(),
		"ScoreTypes":     entities.ValidScoreTypes(),
		"Lifts":          nil,
		"IncludeLifting": includeLifting,
	}
	if lifts != nil {
		data["Lifts"] = lifts.Results
	}
	h.templates.ExecuteTemplate(w, "workouts.html", data)
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
	applyWorkoutFormFields(r, &cmd.TimeCap, &cmd.Rounds, &cmd.IntervalSeconds, &cmd.LiftId)

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

	lifts, _ := h.liftService.GetLiftsByUser(&query.GetLiftsByUserQuery{UserId: userId})
	enrichLiftingWorkout(result.Workout, h.liftService, userId)

	data := map[string]interface{}{
		"Workout":      result.Workout,
		"Results":      result.Results,
		"WorkoutTypes": entities.ValidWorkoutTypes(),
		"ScoreTypes":   entities.ValidScoreTypes(),
		"Lifts":        nil,
	}
	if lifts != nil {
		data["Lifts"] = lifts.Results
	}
	h.templates.ExecuteTemplate(w, "workout_detail.html", data)
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
	applyWorkoutFormFields(r, &cmd.TimeCap, &cmd.Rounds, &cmd.IntervalSeconds, &cmd.LiftId)

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

// applyWorkoutFormFields parses the optional integer/uuid workout form fields
// off the request and assigns them to the target pointers when present.
func applyWorkoutFormFields(r *http.Request, timeCap, rounds, interval **int, liftId **uuid.UUID) {
	parseInt := func(key string, dest **int) {
		if v := r.FormValue(key); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				*dest = &n
			}
		}
	}
	parseInt("time_cap", timeCap)
	parseInt("rounds", rounds)
	parseInt("interval_seconds", interval)

	if v := r.FormValue("lift_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			*liftId = &id
		}
	}
}

// enrichLiftingWorkout fetches the linked Lift for a lifting-type workout and
// populates its 1RM percentage table on the result so templates can render
// them without extra lookups.
func enrichLiftingWorkout(wr *common.WorkoutResult, liftService *services.LiftService, userId uuid.UUID) {
	if wr == nil || wr.Type != string(entities.WorkoutTypeLifting) || wr.LiftId == nil {
		return
	}
	result, err := liftService.GetLiftById(&query.GetLiftByIdQuery{Id: *wr.LiftId, UserId: userId})
	if err != nil || result == nil {
		return
	}
	wr.Lift = result.Lift
	if len(result.PercentageTable) == 0 {
		return
	}
	wr.PercentageTable = result.PercentageTable
	keys := make([]int, 0, len(result.PercentageTable))
	for k := range result.PercentageTable {
		keys = append(keys, k)
	}
	sortInts(keys)
	wr.PctKeys = keys
}

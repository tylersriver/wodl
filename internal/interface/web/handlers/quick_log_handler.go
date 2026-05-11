package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/application/command"
	"github.com/tyler/wodl/internal/application/query"
	"github.com/tyler/wodl/internal/application/services"
	"github.com/tyler/wodl/internal/domain/entities"
	"github.com/tyler/wodl/internal/infrastructure/middleware"
)

// QuickLogHandler renders and submits the streamlined "log a CrossFit day"
// form: pick or inline-create a Lift and a Metcon, fill in today's results,
// and a Session + SessionLog are produced atomically.
type QuickLogHandler struct {
	quickLogService *services.QuickLogService
	liftService     *services.LiftService
	workoutService  *services.WorkoutService
	templates       *template.Template
}

func NewQuickLogHandler(quickLogService *services.QuickLogService, liftService *services.LiftService, workoutService *services.WorkoutService, templates *template.Template) *QuickLogHandler {
	return &QuickLogHandler{
		quickLogService: quickLogService,
		liftService:     liftService,
		workoutService:  workoutService,
		templates:       templates,
	}
}

func (h *QuickLogHandler) Page(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)

	lifts, _ := h.liftService.GetLiftsByUser(&query.GetLiftsByUserQuery{UserId: userId})
	workouts, _ := h.workoutService.GetWorkoutsByUser(&query.GetWorkoutsByUserQuery{UserId: userId})

	data := map[string]interface{}{
		"Today":        time.Now().Format(sessionDateLayout),
		"Lifts":        nil,
		"Workouts":     nil,
		"Categories":   entities.ValidLiftCategories(),
		"WorkoutTypes": metconWorkoutTypes(),
		"ScoreTypes":   entities.ValidScoreTypes(),
	}
	if lifts != nil {
		data["Lifts"] = lifts.Results
	}
	if workouts != nil {
		// Only show non-lifting workouts as "metcon" options. The quick-log
		// form scopes the workout picker to actual conditioning pieces.
		filtered := make([]interface{}, 0, len(workouts.Results))
		for _, wkt := range workouts.Results {
			if wkt.Type == string(entities.WorkoutTypeLifting) {
				continue
			}
			filtered = append(filtered, wkt)
		}
		data["Workouts"] = filtered
	}

	h.templates.ExecuteTemplate(w, "quick_log.html", data)
}

func (h *QuickLogHandler) Submit(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	cmd := &command.QuickLogSessionCommand{
		UserId:           userId,
		Date:             parseSessionDate(r.FormValue("date")),
		TotalTimeMinutes: parseOptionalInt(r.FormValue("total_time_minutes")),
		SessionName:      r.FormValue("session_name"),
		Warmup:           r.FormValue("warmup"),
		Notes:            r.FormValue("notes"),
	}

	if r.FormValue("include_lift") == "1" {
		section, err := buildLiftSection(r)
		if err != nil {
			http.Error(w, "lift: "+err.Error(), http.StatusBadRequest)
			return
		}
		cmd.Lift = section
	}

	if r.FormValue("include_metcon") == "1" {
		section, err := buildMetconSection(r)
		if err != nil {
			http.Error(w, "metcon: "+err.Error(), http.StatusBadRequest)
			return
		}
		cmd.Metcon = section
	}

	result, err := h.quickLogService.QuickLogSession(cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/sessions/%s", result.Session.Id), http.StatusSeeOther)
}

func buildLiftSection(r *http.Request) (*command.QuickLogLiftSection, error) {
	section := &command.QuickLogLiftSection{
		Notes: r.FormValue("lift_notes"),
	}

	weight, err := strconv.ParseFloat(r.FormValue("lift_weight"), 64)
	if err != nil || weight <= 0 {
		return nil, fmt.Errorf("weight required")
	}
	section.Weight = weight

	reps, err := strconv.Atoi(r.FormValue("lift_reps"))
	if err != nil || reps <= 0 {
		return nil, fmt.Errorf("reps required")
	}
	section.Reps = reps

	sets, err := strconv.Atoi(r.FormValue("lift_sets"))
	if err != nil || sets <= 0 {
		return nil, fmt.Errorf("sets required")
	}
	section.Sets = sets

	if v := r.FormValue("lift_rpe"); v != "" {
		if rpe, err := strconv.ParseFloat(v, 64); err == nil {
			section.RPE = &rpe
		}
	}

	mode := r.FormValue("lift_mode")
	if mode == "new" {
		section.NewLiftName = r.FormValue("new_lift_name")
		section.NewLiftCategory = r.FormValue("new_lift_category")
		section.NewLiftOneRepMax = parseOptionalFloat(r.FormValue("new_lift_one_rep_max"))
		return section, nil
	}

	id, err := uuid.Parse(r.FormValue("lift_id"))
	if err != nil {
		return nil, fmt.Errorf("pick a lift or create a new one")
	}
	section.ExistingLiftId = &id
	return section, nil
}

func buildMetconSection(r *http.Request) (*command.QuickLogMetconSection, error) {
	section := &command.QuickLogMetconSection{
		Score:     r.FormValue("metcon_score"),
		ScoreType: r.FormValue("metcon_score_type"),
		Rx:        r.FormValue("metcon_rx") == "on",
		Notes:     r.FormValue("metcon_notes"),
	}

	mode := r.FormValue("metcon_mode")
	if mode == "new" {
		section.NewWorkoutName = r.FormValue("new_workout_name")
		section.NewWorkoutType = r.FormValue("new_workout_type")
		section.NewWorkoutDescription = r.FormValue("new_workout_description")
		section.NewWorkoutTimeCap = parseOptionalInt(r.FormValue("new_workout_time_cap"))
		section.NewWorkoutRounds = parseOptionalInt(r.FormValue("new_workout_rounds"))
		section.NewWorkoutIntervalSeconds = parseOptionalInt(r.FormValue("new_workout_interval_seconds"))
		return section, nil
	}

	id, err := uuid.Parse(r.FormValue("workout_id"))
	if err != nil {
		return nil, fmt.Errorf("pick a workout or create a new one")
	}
	section.ExistingWorkoutId = &id
	return section, nil
}

func parseOptionalInt(v string) *int {
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return nil
	}
	return &n
}

func parseOptionalFloat(v string) *float64 {
	if v == "" {
		return nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil
	}
	return &f
}

// metconWorkoutTypes returns the workout types valid for the metcon picker —
// every type except "lifting", which is reserved for the lift section.
func metconWorkoutTypes() []entities.WorkoutType {
	all := entities.ValidWorkoutTypes()
	out := make([]entities.WorkoutType, 0, len(all))
	for _, t := range all {
		if t == entities.WorkoutTypeLifting {
			continue
		}
		out = append(out, t)
	}
	return out
}

package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/application/command"
	"github.com/tyler/wodl/internal/application/query"
	"github.com/tyler/wodl/internal/application/services"
	"github.com/tyler/wodl/internal/infrastructure/middleware"
)

const sessionDateLayout = "2006-01-02"

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

	sessions, err := h.sessionService.GetSessionsByUser(&query.GetSessionsByUserQuery{UserId: userId})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	workouts, _ := h.workoutService.GetWorkoutsByUser(&query.GetWorkoutsByUserQuery{UserId: userId})

	data := map[string]interface{}{
		"Sessions": sessions.Results,
		"Workouts": nil,
		"Today":    time.Now().Format(sessionDateLayout),
	}
	if workouts != nil {
		data["Workouts"] = workouts.Results
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
	if result.Session != nil && result.Session.Date != nil {
		dateStr = result.Session.Date.Format(sessionDateLayout)
	}

	data := map[string]interface{}{
		"Session":  result.Session,
		"Workouts": nil,
		"DateStr":  dateStr,
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

// parseSessionDate parses a YYYY-MM-DD string from a form input; returns nil
// when empty or malformed.
func parseSessionDate(v string) *time.Time {
	if v == "" {
		return nil
	}
	t, err := time.Parse(sessionDateLayout, v)
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

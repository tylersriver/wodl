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

type LiftHandler struct {
	liftService *services.LiftService
	templates   *template.Template
}

func NewLiftHandler(liftService *services.LiftService, templates *template.Template) *LiftHandler {
	return &LiftHandler{liftService: liftService, templates: templates}
}

func (h *LiftHandler) List(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	result, err := h.liftService.GetLiftsByUser(&query.GetLiftsByUserQuery{UserId: userId})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.templates.ExecuteTemplate(w, "lifts.html", map[string]interface{}{
		"Lifts":      result.Results,
		"Categories": entities.ValidLiftCategories(),
	})
}

func (h *LiftHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	userId := middleware.GetUserID(r)

	var orm *float64
	if v := r.FormValue("one_rep_max"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err == nil && f > 0 {
			orm = &f
		}
	}

	cmd := &command.CreateLiftCommand{
		UserId:    userId,
		Name:      r.FormValue("name"),
		Category:  r.FormValue("category"),
		OneRepMax: orm,
	}

	_, err := h.liftService.CreateLift(cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/lifts", http.StatusSeeOther)
}

func (h *LiftHandler) Detail(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	result, err := h.liftService.GetLiftById(&query.GetLiftByIdQuery{Id: id, UserId: userId})
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Sort percentage table keys for display
	var pctKeys []int
	for k := range result.PercentageTable {
		pctKeys = append(pctKeys, k)
	}
	sortInts(pctKeys)

	h.templates.ExecuteTemplate(w, "lift_detail.html", map[string]interface{}{
		"Lift":            result.Lift,
		"Logs":            result.Logs,
		"PercentageTable": result.PercentageTable,
		"PctKeys":         pctKeys,
		"Categories":      entities.ValidLiftCategories(),
	})
}

func (h *LiftHandler) Update(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	r.ParseForm()
	var orm *float64
	if v := r.FormValue("one_rep_max"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err == nil && f > 0 {
			orm = &f
		}
	}

	cmd := &command.UpdateLiftCommand{
		Id:        id,
		UserId:    userId,
		Name:      r.FormValue("name"),
		Category:  r.FormValue("category"),
		OneRepMax: orm,
	}

	if err := h.liftService.UpdateLift(cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/lifts/%s", id), http.StatusSeeOther)
}

func (h *LiftHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.liftService.DeleteLift(&command.DeleteLiftCommand{Id: id, UserId: userId}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/lifts")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/lifts", http.StatusSeeOther)
}

func (h *LiftHandler) CreateLog(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	liftId, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	r.ParseForm()
	weight, _ := strconv.ParseFloat(r.FormValue("weight"), 64)
	reps, _ := strconv.Atoi(r.FormValue("reps"))
	sets, _ := strconv.Atoi(r.FormValue("sets"))
	if sets == 0 {
		sets = 1
	}

	var rpe *float64
	if v := r.FormValue("rpe"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err == nil {
			rpe = &f
		}
	}

	cmd := &command.CreateLiftLogCommand{
		UserId: userId,
		LiftId: liftId,
		Weight: weight,
		Reps:   reps,
		Sets:   sets,
		RPE:    rpe,
		Notes:  r.FormValue("notes"),
	}

	_, err = h.liftService.CreateLiftLog(cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/lifts/%s", liftId), http.StatusSeeOther)
}

func (h *LiftHandler) DeleteLog(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)
	logId, err := uuid.Parse(chi.URLParam(r, "logId"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	liftId := chi.URLParam(r, "id")

	if err := h.liftService.DeleteLiftLog(&command.DeleteLiftLogCommand{Id: logId, UserId: userId}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", fmt.Sprintf("/lifts/%s", liftId))
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/lifts/%s", liftId), http.StatusSeeOther)
}

func (h *LiftHandler) Calc1RM(w http.ResponseWriter, r *http.Request) {
	weight, _ := strconv.ParseFloat(r.URL.Query().Get("weight"), 64)
	reps, _ := strconv.ParseFloat(r.URL.Query().Get("reps"), 64)

	if weight <= 0 || reps <= 0 {
		w.Write([]byte("--"))
		return
	}

	estimated := entities.EstimateOneRepMax(weight, reps)
	fmt.Fprintf(w, "%.1f", estimated)
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

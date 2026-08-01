package handlers

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tyler/wodl/internal/application/command"
	"github.com/tyler/wodl/internal/application/common"
	"github.com/tyler/wodl/internal/application/services"
	"github.com/tyler/wodl/internal/domain/entities"
	"github.com/tyler/wodl/internal/infrastructure/middleware"
)

const (
	// A handful of phone screenshots, well under the API's 32MB request cap.
	maxImportUploadBytes = 20 << 20
	maxImportImages      = 6
)

type ImportHandler struct {
	importService *services.ImportService
	templates     *template.Template
}

func NewImportHandler(importService *services.ImportService, templates *template.Template) *ImportHandler {
	return &ImportHandler{importService: importService, templates: templates}
}

// Page renders the upload form.
func (h *ImportHandler) Page(w http.ResponseWriter, r *http.Request) {
	if !h.importService.Enabled() {
		http.Redirect(w, r, "/sessions", http.StatusSeeOther)
		return
	}
	h.templates.ExecuteTemplate(w, "import.html", map[string]interface{}{
		"Today": time.Now().Format(sessionDateLayout),
	})
}

// Extract reads the uploaded images and renders an editable review of what was
// found. Nothing is saved here — the user confirms on the next step.
func (h *ImportHandler) Extract(w http.ResponseWriter, r *http.Request) {
	if !h.importService.Enabled() {
		http.Redirect(w, r, "/sessions", http.StatusSeeOther)
		return
	}

	if err := r.ParseMultipartForm(maxImportUploadBytes); err != nil {
		h.renderUploadError(w, "Those images couldn't be read. Try again with smaller files.")
		return
	}
	defer r.MultipartForm.RemoveAll()

	images, err := collectImages(r)
	if err != nil {
		h.renderUploadError(w, err.Error())
		return
	}

	extracted, err := h.importService.Preview(r.Context(), images)
	if err != nil {
		h.renderUploadError(w, fmt.Sprintf("Couldn't read a session from those images: %v", err))
		return
	}
	if len(extracted.Workouts) == 0 {
		h.renderUploadError(w, "No workouts were found in those images. Try a clearer photo.")
		return
	}

	dateStr := extracted.Date.Format(sessionDateLayout)
	if extracted.Date.IsZero() {
		dateStr = time.Now().Format(sessionDateLayout)
	}

	h.templates.ExecuteTemplate(w, "import_review.html", map[string]interface{}{
		"Session":      extracted,
		"DateStr":      dateStr,
		"DateMissing":  extracted.Date.IsZero(),
		"WorkoutTypes": entities.ValidWorkoutTypes(),
		"Categories":   entities.ValidLiftCategories(),
		"Today":        time.Now().Format(sessionDateLayout),
	})
}

// Create saves the reviewed session.
func (h *ImportHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !h.importService.Enabled() {
		http.Redirect(w, r, "/sessions", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	userId := middleware.GetUserID(r)

	cmd := &command.ImportSessionCommand{
		UserId: userId,
		Name:   strings.TrimSpace(r.FormValue("name")),
		Warmup: r.FormValue("warmup"),
		Date:   parseSessionDate(r.FormValue("date")),
	}
	if n, err := strconv.Atoi(r.FormValue("total_time_minutes")); err == nil {
		cmd.TotalTimeMinutes = &n
	}

	// The review form posts parallel arrays, one entry per workout card.
	names := r.Form["workout_name"]
	types := r.Form["workout_type"]
	descriptions := r.Form["workout_description"]
	timeCaps := r.Form["workout_time_cap"]
	rounds := r.Form["workout_rounds"]
	intervals := r.Form["workout_interval"]
	liftNames := r.Form["workout_lift_name"]
	liftCategories := r.Form["workout_lift_category"]

	for i, name := range names {
		if strings.TrimSpace(name) == "" {
			continue
		}
		cmd.Workouts = append(cmd.Workouts, command.ImportWorkout{
			Name:            strings.TrimSpace(name),
			Type:            at(types, i),
			Description:     at(descriptions, i),
			TimeCap:         atInt(timeCaps, i),
			Rounds:          atInt(rounds, i),
			IntervalSeconds: atInt(intervals, i),
			LiftName:        strings.TrimSpace(at(liftNames, i)),
			LiftCategory:    at(liftCategories, i),
		})
	}

	if cmd.Name == "" {
		cmd.Name = defaultSessionName(cmd.Date)
	}

	result, err := h.importService.Create(cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/sessions/%s", result.SessionId), http.StatusSeeOther)
}

func (h *ImportHandler) renderUploadError(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusBadRequest)
	h.templates.ExecuteTemplate(w, "import.html", map[string]interface{}{
		"Error": message,
		"Today": time.Now().Format(sessionDateLayout),
	})
}

// collectImages reads the uploaded files into memory, rejecting anything the
// vision API won't accept before a request is spent on it.
func collectImages(r *http.Request) ([]common.BoardImage, error) {
	files := r.MultipartForm.File["images"]
	if len(files) == 0 {
		return nil, fmt.Errorf("Choose at least one image first.")
	}
	if len(files) > maxImportImages {
		return nil, fmt.Errorf("That's more than %d images — upload the ones for a single day.", maxImportImages)
	}

	images := make([]common.BoardImage, 0, len(files))
	for _, header := range files {
		mediaType := header.Header.Get("Content-Type")
		if !aiSupportedMediaType(mediaType) {
			return nil, fmt.Errorf("%q isn't an image format we can read. Use a JPEG, PNG, GIF or WebP.", header.Filename)
		}
		file, err := header.Open()
		if err != nil {
			return nil, fmt.Errorf("Couldn't open %q.", header.Filename)
		}
		data, err := io.ReadAll(io.LimitReader(file, maxImportUploadBytes))
		file.Close()
		if err != nil {
			return nil, fmt.Errorf("Couldn't read %q.", header.Filename)
		}
		images = append(images, common.BoardImage{MediaType: mediaType, Data: data})
	}
	return images, nil
}

// aiSupportedMediaType mirrors the formats the vision API accepts. Checked here
// so a bad upload is rejected before it reaches the application layer; the
// extractor enforces the same set independently.
func aiSupportedMediaType(mediaType string) bool {
	switch mediaType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

func at(values []string, i int) string {
	if i < len(values) {
		return values[i]
	}
	return ""
}

func atInt(values []string, i int) *int {
	if n, err := strconv.Atoi(strings.TrimSpace(at(values, i))); err == nil {
		return &n
	}
	return nil
}

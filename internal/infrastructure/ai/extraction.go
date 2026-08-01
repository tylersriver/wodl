package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tyler/wodl/internal/application/common"
	"github.com/tyler/wodl/internal/domain/entities"
)

const boardDateLayout = "2006-01-02"

// SupportedMediaType reports whether an uploaded file is an image format the
// vision APIs accept.
func SupportedMediaType(mediaType string) bool {
	switch mediaType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

// boardPayload is the JSON shape both providers are asked to produce. Anthropic
// enforces it with a strict tool schema; Groq is asked for it in the prompt and
// held to it by decodeBoardPayload, so a loose model can't widen the domain.
type boardPayload struct {
	Name             string `json:"name"`
	Date             string `json:"date"`
	Warmup           string `json:"warmup"`
	TotalTimeMinutes *int   `json:"total_time_minutes"`
	Workouts         []struct {
		Name            string `json:"name"`
		Type            string `json:"type"`
		Description     string `json:"description"`
		TimeCapMinutes  *int   `json:"time_cap_minutes"`
		Rounds          *int   `json:"rounds"`
		IntervalSeconds *int   `json:"interval_seconds"`
		LiftName        string `json:"lift_name"`
		LiftCategory    string `json:"lift_category"`
	} `json:"workouts"`
}

// decodeBoardPayload converts a provider's JSON into the application's
// extraction type, discarding anything the domain doesn't accept.
func decodeBoardPayload(raw []byte) (*common.ExtractedSession, error) {
	var payload boardPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("understanding the extracted session: %w", err)
	}

	session := &common.ExtractedSession{
		Name:             strings.TrimSpace(payload.Name),
		Warmup:           strings.TrimSpace(payload.Warmup),
		TotalTimeMinutes: payload.TotalTimeMinutes,
	}
	if d := strings.TrimSpace(payload.Date); d != "" {
		if parsed, err := time.ParseInLocation(boardDateLayout, d, time.Local); err == nil {
			session.Date = parsed
		}
	}

	for _, w := range payload.Workouts {
		if strings.TrimSpace(w.Name) == "" {
			continue
		}
		session.Workouts = append(session.Workouts, &common.ExtractedWorkout{
			Name:            strings.TrimSpace(w.Name),
			Type:            validWorkoutType(w.Type),
			Description:     strings.TrimSpace(w.Description),
			TimeCapMinutes:  w.TimeCapMinutes,
			Rounds:          w.Rounds,
			IntervalSeconds: w.IntervalSeconds,
			LiftName:        strings.TrimSpace(w.LiftName),
			LiftCategory:    validLiftCategory(w.LiftCategory),
		})
	}
	return session, nil
}

// boardPrompt is the shared instruction. describeShape adds an explicit JSON
// contract for providers that have no schema mechanism; Anthropic leaves it off
// because its tool schema already states the shape.
func boardPrompt(now time.Time, describeShape bool) string {
	var b strings.Builder

	fmt.Fprintf(&b, `These images show one day of programming from a CrossFit gym's whiteboard or app.

Record it as a single training session.

Guidance:
- Today is %s (%s). Boards usually print a weekday and a day and month with no
  year — resolve the year so the resulting date matches the weekday shown, and
  prefer the most recent such date rather than a future one. Leave the date
  empty only if the board shows none.
- Each distinct piece of work is its own workout, in the order it appears: a
  strength or barbell piece and a metcon are two workouts, not one.
- Put the movements and reps in the description exactly as the board words
  them, including prescribed loads ("Barbell: 115/80"), scoring notes
  ("Score = Time"), and percentages of anything other than a one-rep max
  ("80-85%% of 5x5"). Do not convert, recalculate, or rephrase these — they are
  kept verbatim so nothing is lost or misstated.
- Use the lifting type only for a piece built around a named barbell lift, and
  set lift_name to that movement.
- Only record numbers the board actually states. Leave a field empty rather
  than guessing.`,
		now.Format("Monday, 2 January 2006"), now.Format(boardDateLayout))

	if describeShape {
		fmt.Fprintf(&b, `

Reply with JSON only — no prose, no code fences — in exactly this shape:

{
  "name": "the board's heading, e.g. \"CrossFit - Fri, Jul 31\"",
  "date": "YYYY-MM-DD, or \"\" if the board shows none",
  "warmup": "warmup text, or \"\"",
  "total_time_minutes": null,
  "workouts": [
    {
      "name": "e.g. \"Storm Warning\"",
      "type": "one of: %s",
      "description": "movements, reps, loads and scoring notes as worded",
      "time_cap_minutes": null,
      "rounds": null,
      "interval_seconds": null,
      "lift_name": "the barbell movement for lifting pieces, else \"\"",
      "lift_category": "one of: %s — or \"\" when not a lifting piece"
    }
  ]
}

Every key must be present. Use null for unknown numbers and "" for unknown
text. interval_seconds is in seconds, so "Every 1:30" is 90.`,
			strings.Join(enumValues(entities.ValidWorkoutTypes()), ", "),
			strings.Join(enumValues(entities.ValidLiftCategories()), ", "))
	}

	return b.String()
}

// mergeExtractions folds per-image results into one session, in image order.
//
// Boards are often split across screenshots — a header with the date on one, a
// metcon on another — so each field is taken from the first image that had it,
// and workouts are concatenated. Workouts repeated across images (a header
// reprinted on both) are matched by name and kept once.
func mergeExtractions(parts []*common.ExtractedSession) *common.ExtractedSession {
	merged := &common.ExtractedSession{}
	seen := map[string]bool{}

	for _, part := range parts {
		if part == nil {
			continue
		}
		if merged.Name == "" {
			merged.Name = part.Name
		}
		if merged.Date.IsZero() {
			merged.Date = part.Date
		}
		if merged.Warmup == "" {
			merged.Warmup = part.Warmup
		}
		if merged.TotalTimeMinutes == nil {
			merged.TotalTimeMinutes = part.TotalTimeMinutes
		}
		for _, w := range part.Workouts {
			key := normalizeKey(w.Name)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			merged.Workouts = append(merged.Workouts, w)
		}
	}
	return merged
}

// normalizeKey collapses case and whitespace so the same workout printed on two
// screenshots isn't imported twice.
func normalizeKey(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func enumValues[T ~string](values []T) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, string(v))
	}
	return out
}

func validWorkoutType(v string) string {
	for _, t := range entities.ValidWorkoutTypes() {
		if string(t) == v {
			return v
		}
	}
	return string(entities.WorkoutTypeCustom)
}

func validLiftCategory(v string) string {
	for _, c := range entities.ValidLiftCategories() {
		if string(c) == v {
			return v
		}
	}
	return ""
}

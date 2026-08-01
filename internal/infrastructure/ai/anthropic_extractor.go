// Package ai holds the outward-facing AI integrations. It implements ports
// declared in the application layer, so nothing above it depends on a vendor.
package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/tyler/wodl/internal/application/common"
	"github.com/tyler/wodl/internal/domain/entities"
)

const (
	extractModel = anthropic.Model("claude-opus-5")
	// Extraction output is small; the headroom is for thinking, which counts
	// against the same budget.
	extractMaxTokens = 8000
	extractToolName  = "record_session"
	boardDateLayout  = "2006-01-02"
)

// Extractor reads workout boards using Claude's vision support.
type Extractor struct {
	client anthropic.Client
	// now is injected so tests get a fixed clock; boards print a weekday and a
	// day-month with no year, so resolving a date depends on today.
	now func() time.Time
}

// NewExtractor returns an Extractor, or nil when no API key is configured so
// the caller can disable the feature instead of failing at request time.
func NewExtractor(apiKey string) *Extractor {
	if strings.TrimSpace(apiKey) == "" {
		return nil
	}
	return &Extractor{
		client: anthropic.NewClient(
			option.WithAPIKey(apiKey),
			// A person is waiting on this call, so fail well before the SDK's
			// ten-minute default.
			option.WithRequestTimeout(90*time.Second),
		),
		now: time.Now,
	}
}

// SupportedMediaType reports whether an uploaded file is an image format the
// API accepts.
func SupportedMediaType(mediaType string) bool {
	switch mediaType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

func (e *Extractor) Extract(ctx context.Context, images []common.BoardImage) (*common.ExtractedSession, error) {
	blocks := make([]anthropic.ContentBlockParamUnion, 0, len(images)+1)
	for _, img := range images {
		if !SupportedMediaType(img.MediaType) {
			return nil, fmt.Errorf("unsupported image type %q", img.MediaType)
		}
		blocks = append(blocks, anthropic.NewImageBlockBase64(
			img.MediaType,
			base64.StdEncoding.EncodeToString(img.Data),
		))
	}
	blocks = append(blocks, anthropic.NewTextBlock(e.prompt()))

	adaptive := anthropic.ThinkingConfigAdaptiveParam{}
	tool := anthropic.ToolParam{
		Name:        extractToolName,
		Description: anthropic.String("Record the training session shown on the board image(s)."),
		Strict:      anthropic.Bool(true),
		InputSchema: sessionSchema(),
	}

	message, err := e.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     extractModel,
		MaxTokens: extractMaxTokens,
		Thinking:  anthropic.ThinkingConfigParamUnion{OfAdaptive: &adaptive},
		// Extraction accuracy is the whole feature, so this stays at the API
		// default rather than trading it away for latency. It is the knob to
		// turn if imports feel slow.
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortHigh},
		Tools:        []anthropic.ToolUnionParam{{OfTool: &tool}},
		ToolChoice:   anthropic.ToolChoiceParamOfTool(extractToolName),
		Messages:     []anthropic.MessageParam{anthropic.NewUserMessage(blocks...)},
	})
	if err != nil {
		return nil, fmt.Errorf("reading the board: %w", err)
	}
	if message.StopReason == anthropic.StopReasonRefusal {
		return nil, fmt.Errorf("the request was declined")
	}

	for _, block := range message.Content {
		if use, ok := block.AsAny().(anthropic.ToolUseBlock); ok && use.Name == extractToolName {
			return e.decode(use.Input)
		}
	}
	return nil, fmt.Errorf("no session could be read from the image")
}

func (e *Extractor) prompt() string {
	today := e.now()
	return fmt.Sprintf(`These images show one day of programming from a CrossFit gym's whiteboard or app.

Record it as a single training session by calling %s.

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
  than guessing.`, extractToolName, today.Format("Monday, 2 January 2006"), today.Format(boardDateLayout))
}

// decode converts the tool payload into the application's extraction type,
// dropping anything that isn't a value the domain accepts.
func (e *Extractor) decode(raw json.RawMessage) (*common.ExtractedSession, error) {
	var payload struct {
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

// sessionSchema mirrors the domain's enums so the model can only produce values
// the app accepts. Strict tool use requires every property to be listed in
// required and additionalProperties to be false, so optional fields are
// expressed as nullable rather than omitted.
func sessionSchema() anthropic.ToolInputSchemaParam {
	nullableInt := map[string]any{"type": []string{"integer", "null"}}

	workout := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required": []string{
			"name", "type", "description",
			"time_cap_minutes", "rounds", "interval_seconds",
			"lift_name", "lift_category",
		},
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "The workout's name as written, e.g. \"Storm Warning\". For an unnamed piece, a short descriptive name.",
			},
			"type": map[string]any{
				"type":        "string",
				"enum":        enumValues(entities.ValidWorkoutTypes()),
				"description": "Closest matching format. Use lifting for a named barbell strength piece, custom when nothing else fits.",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "The movements, reps and any prescribed loads or scoring notes, worded as the board words them.",
			},
			"time_cap_minutes": nullableInt,
			"rounds":           nullableInt,
			"interval_seconds": map[string]any{
				"type":        []string{"integer", "null"},
				"description": "Seconds between intervals for EMOM-style work, e.g. 90 for \"Every 1:30\".",
			},
			"lift_name": map[string]any{
				"type":        "string",
				"description": "For lifting pieces, the barbell movement, e.g. \"Sumo Deadlift\". Empty otherwise.",
			},
			"lift_category": map[string]any{
				"type":        "string",
				"enum":        append(enumValues(entities.ValidLiftCategories()), ""),
				"description": "The lift's category, or empty when this is not a lifting piece.",
			},
		},
	}

	return anthropic.ToolInputSchemaParam{
		Required: []string{"name", "date", "warmup", "total_time_minutes", "workouts"},
		Properties: map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "The session's heading, e.g. \"CrossFit - Fri, Jul 31\".",
			},
			"date": map[string]any{
				"type":        "string",
				"description": "The day the session is for, as YYYY-MM-DD. Empty if the board shows no date.",
			},
			"warmup": map[string]any{
				"type":        "string",
				"description": "Warmup text if the board shows one, otherwise empty.",
			},
			"total_time_minutes": nullableInt,
			"workouts": map[string]any{
				"type":        "array",
				"items":       workout,
				"description": "Each piece of work, in the order it appears on the board.",
			},
		},
		ExtraFields: map[string]any{"additionalProperties": false},
	}
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

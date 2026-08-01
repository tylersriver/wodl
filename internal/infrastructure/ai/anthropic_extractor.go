// Package ai holds the outward-facing AI integrations. Each implements the
// services.BoardExtractor port declared in the application layer, so nothing
// above this package depends on a particular vendor.
package ai

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/tyler/wodl/internal/application/common"
	"github.com/tyler/wodl/internal/domain/entities"
)

const (
	anthropicModel = anthropic.Model("claude-opus-5")
	// Extraction output is small; the headroom is for thinking, which counts
	// against the same budget.
	anthropicMaxTokens = 8000
	extractToolName    = "record_session"
)

// AnthropicExtractor reads workout boards using Claude's vision support and a
// strict tool schema, so the model can only return values the domain accepts.
type AnthropicExtractor struct {
	client anthropic.Client
	// now is injected so tests get a fixed clock; boards print a weekday and a
	// day-month with no year, so resolving a date depends on today.
	now func() time.Time
}

// NewAnthropicExtractor returns an extractor, or nil when no API key is
// configured so the caller can disable the feature rather than fail later.
func NewAnthropicExtractor(apiKey string) *AnthropicExtractor {
	if strings.TrimSpace(apiKey) == "" {
		return nil
	}
	return &AnthropicExtractor{
		client: anthropic.NewClient(
			option.WithAPIKey(apiKey),
			// A person is waiting on this call, so fail well before the SDK's
			// ten-minute default.
			option.WithRequestTimeout(90*time.Second),
		),
		now: time.Now,
	}
}

// buildParams assembles the request. Split out from Extract so a test can
// validate the exact tool schema and image blocks against the API's validator
// without spending a completion.
func (e *AnthropicExtractor) buildParams(images []common.BoardImage) (anthropic.MessageNewParams, error) {
	blocks := make([]anthropic.ContentBlockParamUnion, 0, len(images)+1)
	for _, img := range images {
		if !SupportedMediaType(img.MediaType) {
			return anthropic.MessageNewParams{}, fmt.Errorf("unsupported image type %q", img.MediaType)
		}
		blocks = append(blocks, anthropic.NewImageBlockBase64(
			img.MediaType,
			base64.StdEncoding.EncodeToString(img.Data),
		))
	}
	// The tool schema already states the shape, so the prompt doesn't repeat it.
	blocks = append(blocks, anthropic.NewTextBlock(boardPrompt(e.now(), false)))

	adaptive := anthropic.ThinkingConfigAdaptiveParam{}
	tool := anthropic.ToolParam{
		Name:        extractToolName,
		Description: anthropic.String("Record the training session shown on the board image(s)."),
		// Deliberately not Strict. With strict decoding on, this schema made the
		// model's own tool-call framing leak into the payload: the board was read
		// correctly, but the whole workouts array arrived as literal text inside
		// the preceding "warmup" string and workouts itself came back empty — so
		// an import failed with "no workouts found" on a perfectly legible photo.
		// Reproduced on every strict run across two models; clean on every
		// non-strict one. The schema below still guides the model, and
		// decodeBoardPayload is what actually holds the result to the domain.
		InputSchema: sessionSchema(),
	}

	return anthropic.MessageNewParams{
		Model:     anthropicModel,
		MaxTokens: anthropicMaxTokens,
		Thinking:  anthropic.ThinkingConfigParamUnion{OfAdaptive: &adaptive},
		// Extraction accuracy is the whole feature, so this stays at the API
		// default rather than trading it away for latency. It is the knob to
		// turn if imports feel slow.
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortHigh},
		Tools:        []anthropic.ToolUnionParam{{OfTool: &tool}},
		ToolChoice:   anthropic.ToolChoiceParamOfTool(extractToolName),
		Messages:     []anthropic.MessageParam{anthropic.NewUserMessage(blocks...)},
	}, nil
}

func (e *AnthropicExtractor) Extract(ctx context.Context, images []common.BoardImage) (*common.ExtractedSession, error) {
	params, err := e.buildParams(images)
	if err != nil {
		return nil, err
	}

	message, err := e.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("reading the board: %w", err)
	}
	if message.StopReason == anthropic.StopReasonRefusal {
		return nil, fmt.Errorf("the request was declined")
	}

	for _, block := range message.Content {
		if use, ok := block.AsAny().(anthropic.ToolUseBlock); ok && use.Name == extractToolName {
			return decodeBoardPayload(use.Input)
		}
	}
	return nil, fmt.Errorf("no session could be read from the image")
}

// sessionSchema mirrors the domain's enums so the model is steered towards
// values the app accepts. It is guidance, not enforcement — see the note on
// Strict in buildParams — so every field is listed as required and optional
// ones are expressed as nullable, which keeps the shape unambiguous without
// relying on the decoder to reject anything. validWorkoutType and
// validLiftCategory are what actually hold the result to the domain.
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

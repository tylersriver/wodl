package ai

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/tyler/wodl/internal/application/common"
)

// TestExtractor_ReadsBoardImages calls the real API, so it is skipped unless
// both a key and fixture images are provided:
//
//	ANTHROPIC_API_KEY=... WODL_BOARD_FIXTURES=/path/a.jpeg,/path/b.jpeg \
//	  go test ./internal/infrastructure/ai/ -run ReadsBoard -v
//
// Fixtures are passed by path rather than committed, so real gym screenshots
// don't have to live in the repo.
func TestExtractor_ReadsBoardImages(t *testing.T) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	fixtures := os.Getenv("WODL_BOARD_FIXTURES")
	if key == "" || fixtures == "" {
		t.Skip("set ANTHROPIC_API_KEY and WODL_BOARD_FIXTURES to run the live extraction test")
	}

	extractor := NewAnthropicExtractor(key)
	if extractor == nil {
		t.Fatal("expected an extractor for a non-empty key")
	}
	// Pin the clock so year resolution is reproducible across runs.
	extractor.now = func() time.Time {
		return time.Date(2026, 8, 1, 9, 0, 0, 0, time.Local)
	}

	var images []common.BoardImage
	for _, path := range strings.Split(fixtures, ",") {
		path = strings.TrimSpace(path)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading fixture %s: %v", path, err)
		}
		images = append(images, common.BoardImage{MediaType: mediaTypeFor(path), Data: data})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	start := time.Now()
	session, err := extractor.Extract(ctx, images)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	t.Logf("took %s for %d image(s)", elapsed.Round(time.Millisecond), len(images))
	t.Logf("session name:  %q", session.Name)
	if session.Date.IsZero() {
		t.Logf("session date:  (none read)")
	} else {
		t.Logf("session date:  %s", session.Date.Format("2006-01-02 (Mon)"))
	}
	t.Logf("warmup:        %q", session.Warmup)
	t.Logf("total minutes: %s", optInt(session.TotalTimeMinutes))

	for i, w := range session.Workouts {
		t.Logf("--- workout %d ---", i+1)
		t.Logf("  name:     %q", w.Name)
		t.Logf("  type:     %q", w.Type)
		t.Logf("  rounds:   %s   time cap: %s   interval: %s",
			optInt(w.Rounds), optInt(w.TimeCapMinutes), optInt(w.IntervalSeconds))
		t.Logf("  lift:     %q (%s)", w.LiftName, w.LiftCategory)
		t.Logf("  description:\n%s", indent(w.Description))
	}

	if len(session.Workouts) == 0 {
		t.Fatal("expected at least one workout")
	}
	for i, w := range session.Workouts {
		if strings.TrimSpace(w.Name) == "" {
			t.Errorf("workout %d has no name", i+1)
		}
		if strings.TrimSpace(w.Type) == "" {
			t.Errorf("workout %d has no type", i+1)
		}
	}
}

// TestExtractor_RequestShapeIsAccepted validates the tool schema and image
// blocks against the API's own validator via token counting, which does not
// consume a completion. Cheaper than the extraction test above and the first
// thing to run when changing the schema.
func TestExtractor_RequestShapeIsAccepted(t *testing.T) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	fixtures := os.Getenv("WODL_BOARD_FIXTURES")
	if key == "" || fixtures == "" {
		t.Skip("set ANTHROPIC_API_KEY and WODL_BOARD_FIXTURES to run the live request-shape test")
	}

	extractor := NewAnthropicExtractor(key)
	var images []common.BoardImage
	for _, path := range strings.Split(fixtures, ",") {
		path = strings.TrimSpace(path)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading fixture %s: %v", path, err)
		}
		images = append(images, common.BoardImage{MediaType: mediaTypeFor(path), Data: data})
	}

	params, err := extractor.buildParams(images)
	if err != nil {
		t.Fatalf("building params: %v", err)
	}

	tools := make([]anthropic.MessageCountTokensToolUnionParam, 0, len(params.Tools))
	for _, tool := range params.Tools {
		tools = append(tools, anthropic.MessageCountTokensToolUnionParam{OfTool: tool.OfTool})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	count, err := extractor.client.Messages.CountTokens(ctx, anthropic.MessageCountTokensParams{
		Model:      params.Model,
		Messages:   params.Messages,
		Thinking:   params.Thinking,
		Tools:      tools,
		ToolChoice: params.ToolChoice,
	})
	if err != nil {
		t.Fatalf("the API rejected the request shape: %v", err)
	}

	t.Logf("request accepted: %d input tokens for %d image(s)", count.InputTokens, len(images))
	if count.InputTokens <= 0 {
		t.Fatal("expected a positive token count")
	}
}

func optInt(v *int) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprint(*v)
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "      " + l
	}
	return strings.Join(lines, "\n")
}

func mediaTypeFor(path string) string {
	switch {
	case strings.HasSuffix(strings.ToLower(path), ".png"):
		return "image/png"
	case strings.HasSuffix(strings.ToLower(path), ".webp"):
		return "image/webp"
	case strings.HasSuffix(strings.ToLower(path), ".gif"):
		return "image/gif"
	default:
		return "image/jpeg"
	}
}

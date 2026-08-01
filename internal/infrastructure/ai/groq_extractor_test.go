package ai

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tyler/wodl/internal/application/common"
)

func fixedClock() func() time.Time {
	return func() time.Time { return time.Date(2026, 8, 1, 9, 0, 0, 0, time.Local) }
}

func TestGroqExtractor_DisabledWithoutKey(t *testing.T) {
	if NewGroqExtractor("", "") != nil {
		t.Fatal("expected nil extractor without a key")
	}
	if NewGroqExtractor("   ", "") != nil {
		t.Fatal("expected nil extractor for a blank key")
	}
}

func TestGroqExtractor_ModelIsConfigurable(t *testing.T) {
	if got := NewGroqExtractor("k", "").Model(); got != DefaultGroqModel {
		t.Errorf("empty model should fall back to the default, got %q", got)
	}
	if got := NewGroqExtractor("k", "some/other-model").Model(); got != "some/other-model" {
		t.Errorf("model override ignored, got %q", got)
	}
}

// TestGroqExtractor_BuildRequest checks the wire shape, since it can't be
// validated against the live API from here.
func TestGroqExtractor_BuildRequest(t *testing.T) {
	e := NewGroqExtractor("k", "")
	e.now = fixedClock()

	req, err := e.buildRequest([]common.BoardImage{
		{MediaType: "image/jpeg", Data: []byte{0xff, 0xd8}},
		{MediaType: "image/png", Data: []byte{0x89, 0x50}},
	})
	if err != nil {
		t.Fatalf("buildRequest: %v", err)
	}

	if req.Temperature != 0 {
		t.Errorf("extraction should be deterministic, got temperature %v", req.Temperature)
	}
	if req.ResponseFormat["type"] != "json_object" {
		t.Errorf("expected JSON mode, got %v", req.ResponseFormat)
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != "user" {
		t.Fatalf("expected a single user message, got %+v", req.Messages)
	}

	content := req.Messages[0].Content
	if len(content) != 3 {
		t.Fatalf("expected two images plus the prompt, got %d parts", len(content))
	}
	for i, want := range []string{"data:image/jpeg;base64,", "data:image/png;base64,"} {
		if content[i].Type != "image_url" {
			t.Errorf("part %d should be an image, got %q", i, content[i].Type)
		}
		if !strings.HasPrefix(content[i].ImageURL.URL, want) {
			t.Errorf("part %d should be a %s data URL, got %.30s", i, want, content[i].ImageURL.URL)
		}
	}

	prompt := content[2].Text
	if content[2].Type != "text" {
		t.Fatalf("last part should be the prompt, got %q", content[2].Type)
	}
	// Without a schema mechanism the prompt has to carry the contract.
	for _, want := range []string{"2026-08-01", "for_time", "deadlift", "interval_seconds", "verbatim"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt is missing %q", want)
		}
	}

	// The whole payload must marshal — a nil ImageURL or bad tag would surface here.
	if _, err := json.Marshal(req); err != nil {
		t.Fatalf("request does not marshal: %v", err)
	}
}

func TestGroqExtractor_RejectsUnsupportedImage(t *testing.T) {
	e := NewGroqExtractor("k", "")
	_, err := e.buildRequest([]common.BoardImage{{MediaType: "application/pdf", Data: []byte("x")}})
	if err == nil {
		t.Fatal("expected an error for a non-image upload")
	}
}

// TestDecodeBoardPayload_ClampsToDomain is the safety property that lets a
// loosely-schema'd provider be trusted: whatever comes back, only values the
// domain accepts survive.
func TestDecodeBoardPayload_ClampsToDomain(t *testing.T) {
	raw := []byte(`{
      "name": "  CrossFit - Fri, Jul 31  ",
      "date": "2026-07-31",
      "warmup": "",
      "total_time_minutes": 60,
      "workouts": [
        {"name": "Storm Warning", "type": "for_time", "description": " Barbell: 115/80 ",
         "time_cap_minutes": 15, "rounds": 3, "interval_seconds": null,
         "lift_name": "", "lift_category": ""},
        {"name": "Sumo Deadlift", "type": "strength-emom", "description": "x",
         "time_cap_minutes": null, "rounds": 6, "interval_seconds": 90,
         "lift_name": " Sumo Deadlift ", "lift_category": "posterior-chain"},
        {"name": "   ", "type": "amrap", "description": "dropped: no name",
         "time_cap_minutes": null, "rounds": null, "interval_seconds": null,
         "lift_name": "", "lift_category": ""}
      ]
    }`)

	session, err := decodeBoardPayload(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if session.Name != "CrossFit - Fri, Jul 31" {
		t.Errorf("name should be trimmed, got %q", session.Name)
	}
	if got := session.Date.Format("2006-01-02"); got != "2026-07-31" {
		t.Errorf("date = %s", got)
	}
	if len(session.Workouts) != 2 {
		t.Fatalf("the unnamed workout should be dropped, got %d", len(session.Workouts))
	}

	if session.Workouts[0].Type != "for_time" {
		t.Errorf("a valid type should survive, got %q", session.Workouts[0].Type)
	}
	if session.Workouts[0].Description != "Barbell: 115/80" {
		t.Errorf("board wording should be kept and trimmed, got %q", session.Workouts[0].Description)
	}

	// An invented type falls back to custom rather than reaching the domain.
	if session.Workouts[1].Type != "custom" {
		t.Errorf("unknown type should clamp to custom, got %q", session.Workouts[1].Type)
	}
	// An invented category is dropped, not passed through.
	if session.Workouts[1].LiftCategory != "" {
		t.Errorf("unknown category should be dropped, got %q", session.Workouts[1].LiftCategory)
	}
	if session.Workouts[1].LiftName != "Sumo Deadlift" {
		t.Errorf("lift name should be trimmed, got %q", session.Workouts[1].LiftName)
	}
}

func TestDecodeBoardPayload_MissingDateIsZero(t *testing.T) {
	session, err := decodeBoardPayload([]byte(`{"name":"x","date":"","warmup":"","total_time_minutes":null,
      "workouts":[{"name":"a","type":"amrap","description":"","time_cap_minutes":null,
      "rounds":null,"interval_seconds":null,"lift_name":"","lift_category":""}]}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !session.Date.IsZero() {
		t.Errorf("a missing date should stay zero so the UI can flag it, got %s", session.Date)
	}
}

func TestStripCodeFence(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"{\"a\":1}", `{"a":1}`},
		{"```json\n{\"a\":1}\n```", `{"a":1}`},
		{"```\n{\"a\":1}\n```", `{"a":1}`},
		{"  {\"a\":1}  ", `{"a":1}`},
	} {
		if got := stripCodeFence(tc.in); got != tc.want {
			t.Errorf("stripCodeFence(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestGroqExtractor_ReadsBoardImages calls the real API, so it is skipped
// unless a key and fixtures are provided:
//
//	GROQ_API_KEY=... WODL_BOARD_FIXTURES=/path/a.jpeg,/path/b.jpeg \
//	  go test ./internal/infrastructure/ai/ -run Groq_ReadsBoard -v
func TestGroqExtractor_ReadsBoardImages(t *testing.T) {
	key := os.Getenv("GROQ_API_KEY")
	fixtures := os.Getenv("WODL_BOARD_FIXTURES")
	if key == "" || fixtures == "" {
		t.Skip("set GROQ_API_KEY and WODL_BOARD_FIXTURES to run the live Groq test")
	}

	e := NewGroqExtractor(key, os.Getenv("GROQ_MODEL"))
	e.now = fixedClock()
	t.Logf("model: %s", e.Model())

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
	session, err := e.Extract(ctx, images)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	t.Logf("took %s for %d image(s)", time.Since(start).Round(time.Millisecond), len(images))
	t.Logf("session name:  %q", session.Name)
	if session.Date.IsZero() {
		t.Logf("session date:  (none read)")
	} else {
		t.Logf("session date:  %s", session.Date.Format("2006-01-02 (Mon)"))
	}
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
}

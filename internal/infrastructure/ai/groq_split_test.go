package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tyler/wodl/internal/application/common"
)

// fakeGroq stands in for the API so the real request path can be exercised
// offline: same HTTP client, same encoding, same response handling.
type fakeGroq struct {
	mu       sync.Mutex
	requests []groqRequest
	replies  []string
	status   int
	server   *httptest.Server
}

func newFakeGroq(t *testing.T, replies ...string) *fakeGroq {
	t.Helper()
	f := &fakeGroq{replies: replies, status: http.StatusOK}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req groqRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decoding request: %v", err)
		}

		f.mu.Lock()
		n := len(f.requests)
		f.requests = append(f.requests, req)
		status, replies := f.status, f.replies
		f.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if status != http.StatusOK {
			w.WriteHeader(status)
			fmt.Fprint(w, replies[0])
			return
		}
		reply := `{}`
		if n < len(replies) {
			reply = replies[n]
		}
		fmt.Fprintf(w, `{"choices":[{"message":{"content":%s}}]}`, strconv.Quote(reply))
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeGroq) extractor() *GroqExtractor {
	e := NewGroqExtractor("test-key", "test-model", 0)
	e.baseURL = f.server.URL
	e.now = fixedClock()
	return e
}

func (f *fakeGroq) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func board(name, date string, workouts ...string) string {
	parts := make([]string, 0, len(workouts))
	for _, w := range workouts {
		parts = append(parts, fmt.Sprintf(`{"name":%q,"type":"amrap","description":"d",
          "time_cap_minutes":null,"rounds":null,"interval_seconds":null,
          "lift_name":"","lift_category":""}`, w))
	}
	return fmt.Sprintf(`{"name":%q,"date":%q,"warmup":"","total_time_minutes":null,"workouts":[%s]}`,
		name, date, strings.Join(parts, ","))
}

// TestGroqExtractor_SendsOneRequestPerImage is the fix for the free tier's
// per-minute token limit: a single request carrying both screenshots exceeds
// the whole budget and is rejected outright, while two smaller ones fit.
func TestGroqExtractor_SendsOneRequestPerImage(t *testing.T) {
	fake := newFakeGroq(t,
		board("CrossFit - Fri, Jul 31", "2026-07-31", "Sumo Deadlift"),
		board("", "", "Storm Warning"),
	)

	session, err := fake.extractor().Extract(context.Background(), []common.BoardImage{
		{MediaType: "image/jpeg", Data: []byte{0xff, 0xd8}},
		{MediaType: "image/png", Data: []byte{0x89, 0x50}},
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if got := fake.count(); got != 2 {
		t.Fatalf("expected one request per image, got %d", got)
	}
	for i, req := range fake.requests {
		images := 0
		for _, c := range req.Messages[0].Content {
			if c.Type == "image_url" {
				images++
			}
		}
		if images != 1 {
			t.Errorf("request %d carried %d images, want exactly 1", i+1, images)
		}
	}

	// The header from the first image and the metcon from the second combine.
	if session.Name != "CrossFit - Fri, Jul 31" {
		t.Errorf("name should come from the first image that had one, got %q", session.Name)
	}
	if got := session.Date.Format("2006-01-02"); got != "2026-07-31" {
		t.Errorf("date should carry over from the first image, got %s", got)
	}
	if len(session.Workouts) != 2 {
		t.Fatalf("expected both workouts, got %d", len(session.Workouts))
	}
	if session.Workouts[0].Name != "Sumo Deadlift" || session.Workouts[1].Name != "Storm Warning" {
		t.Errorf("workouts should keep image order, got %q then %q",
			session.Workouts[0].Name, session.Workouts[1].Name)
	}
}

func TestGroqExtractor_SingleImageSendsOneRequest(t *testing.T) {
	fake := newFakeGroq(t, board("A day", "2026-07-31", "Fran"))

	if _, err := fake.extractor().Extract(context.Background(),
		[]common.BoardImage{{MediaType: "image/jpeg", Data: []byte{0xff, 0xd8}}}); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if got := fake.count(); got != 1 {
		t.Errorf("expected 1 request, got %d", got)
	}
}

// TestGroqExtractor_NamesTheFailingImage keeps a partial failure diagnosable:
// with several uploads, the message should say which one broke.
func TestGroqExtractor_NamesTheFailingImage(t *testing.T) {
	fake := newFakeGroq(t, `{"error":{"message":"Request too large ... tokens per minute (TPM): Limit 8000","type":"rate_limit"}}`)
	fake.status = http.StatusTooManyRequests

	_, err := fake.extractor().Extract(context.Background(), []common.BoardImage{
		{MediaType: "image/jpeg", Data: []byte{0xff, 0xd8}},
		{MediaType: "image/png", Data: []byte{0x89, 0x50}},
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "image 1 of 2") {
		t.Errorf("error should name the failing image, got: %v", err)
	}
	// And it should still tell the user what to do about it.
	if !strings.Contains(err.Error(), "GROQ_MAX_IMAGE_EDGE") {
		t.Errorf("rate-limit errors should carry the fix, got: %v", err)
	}
}

func TestGroqExtractor_StopsAfterAFailedImage(t *testing.T) {
	fake := newFakeGroq(t, `{"error":{"message":"boom","type":"server_error"}}`)
	fake.status = http.StatusInternalServerError

	_, err := fake.extractor().Extract(context.Background(), []common.BoardImage{
		{MediaType: "image/jpeg", Data: []byte{0xff, 0xd8}},
		{MediaType: "image/png", Data: []byte{0x89, 0x50}},
		{MediaType: "image/png", Data: []byte{0x89, 0x50}},
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := fake.count(); got != 1 {
		t.Errorf("should stop at the first failure rather than burning budget, got %d requests", got)
	}
}

func TestMergeExtractions(t *testing.T) {
	t.Run("first non-empty field wins", func(t *testing.T) {
		sixty := 60
		merged := mergeExtractions([]*common.ExtractedSession{
			{Name: "", Warmup: "", Workouts: []*common.ExtractedWorkout{{Name: "A"}}},
			{Name: "Second", Date: time.Date(2026, 7, 31, 0, 0, 0, 0, time.Local),
				Warmup: "row 5 min", TotalTimeMinutes: &sixty,
				Workouts: []*common.ExtractedWorkout{{Name: "B"}}},
		})
		if merged.Name != "Second" {
			t.Errorf("name = %q", merged.Name)
		}
		if merged.Warmup != "row 5 min" {
			t.Errorf("warmup = %q", merged.Warmup)
		}
		if merged.TotalTimeMinutes == nil || *merged.TotalTimeMinutes != 60 {
			t.Errorf("total time not carried over")
		}
		if merged.Date.IsZero() {
			t.Error("date not carried over")
		}
	})

	t.Run("repeated workouts are kept once", func(t *testing.T) {
		merged := mergeExtractions([]*common.ExtractedSession{
			{Workouts: []*common.ExtractedWorkout{{Name: "Storm Warning"}, {Name: "Sumo Deadlift"}}},
			{Workouts: []*common.ExtractedWorkout{{Name: "  storm   warning "}, {Name: "Cooldown"}}},
		})
		if len(merged.Workouts) != 3 {
			t.Fatalf("expected 3 unique workouts, got %d", len(merged.Workouts))
		}
		if merged.Workouts[2].Name != "Cooldown" {
			t.Errorf("unexpected third workout %q", merged.Workouts[2].Name)
		}
	})

	t.Run("nil parts are skipped", func(t *testing.T) {
		merged := mergeExtractions([]*common.ExtractedSession{nil, {Name: "X"}, nil})
		if merged.Name != "X" {
			t.Errorf("name = %q", merged.Name)
		}
	})
}

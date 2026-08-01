package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tyler/wodl/internal/application/common"
)

const (
	groqBaseURL = "https://api.groq.com/openai/v1"
	// DefaultGroqModel is a vision-capable model on Groq. Groq's catalogue
	// turns over quickly, so this is only a default — override it with
	// GROQ_MODEL, and list what your key can actually reach with:
	//
	//	curl https://api.groq.com/openai/v1/models -H "Authorization: Bearer $GROQ_API_KEY"
	DefaultGroqModel = "meta-llama/llama-4-scout-17b-16e-instruct"
	groqMaxTokens    = 4000
	// DefaultGroqMaxImageEdge caps how large an image is sent. Vision models
	// bill by pixel area, and Groq's free tier allows only 8,000 tokens per
	// minute — two full-resolution phone screenshots exceed that on their own.
	// 1024px keeps a whiteboard legible while fitting a pair inside the budget.
	// Override with GROQ_MAX_IMAGE_EDGE; 0 disables scaling.
	DefaultGroqMaxImageEdge = 1024
)

// GroqExtractor reads workout boards through Groq's OpenAI-compatible
// chat-completions endpoint.
//
// It asks for JSON rather than using tool calling: Groq's vision models have
// historically not supported tools and images in the same request, and JSON
// mode works on both. The shape is stated in the prompt and enforced on the way
// back by decodeBoardPayload, which discards anything outside the domain's
// enums — so a looser model can't widen what the app accepts.
type GroqExtractor struct {
	apiKey  string
	model   string
	maxEdge int
	client  *http.Client
	now     func() time.Time
}

// NewGroqExtractor returns an extractor, or nil when no API key is configured.
// An empty model falls back to DefaultGroqModel; a maxEdge below zero falls
// back to DefaultGroqMaxImageEdge, and exactly zero disables image scaling.
func NewGroqExtractor(apiKey, model string, maxEdge int) *GroqExtractor {
	if strings.TrimSpace(apiKey) == "" {
		return nil
	}
	if strings.TrimSpace(model) == "" {
		model = DefaultGroqModel
	}
	if maxEdge < 0 {
		maxEdge = DefaultGroqMaxImageEdge
	}
	return &GroqExtractor{
		apiKey:  apiKey,
		model:   model,
		maxEdge: maxEdge,
		// A person is waiting on this call.
		client: &http.Client{Timeout: 90 * time.Second},
		now:    time.Now,
	}
}

// Model reports which model this extractor will call, for logging and tests.
func (e *GroqExtractor) Model() string { return e.model }

type groqRequest struct {
	Model          string            `json:"model"`
	MaxTokens      int               `json:"max_tokens"`
	Temperature    float64           `json:"temperature"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
	Messages       []groqMessage     `json:"messages"`
}

type groqMessage struct {
	Role    string        `json:"role"`
	Content []groqContent `json:"content"`
}

type groqContent struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *groqImageURL `json:"image_url,omitempty"`
}

type groqImageURL struct {
	URL string `json:"url"`
}

type groqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// buildRequest assembles the payload. Split out from Extract so it can be
// asserted on without a network call.
func (e *GroqExtractor) buildRequest(images []common.BoardImage) (*groqRequest, error) {
	content := make([]groqContent, 0, len(images)+1)
	for _, img := range images {
		if !SupportedMediaType(img.MediaType) {
			return nil, fmt.Errorf("unsupported image type %q", img.MediaType)
		}
		// Scale before encoding: a full-resolution screenshot can exceed a
		// whole minute's token budget by itself.
		sized, err := shrinkToFit(img, e.maxEdge)
		if err != nil {
			return nil, err
		}
		content = append(content, groqContent{
			Type: "image_url",
			ImageURL: &groqImageURL{URL: fmt.Sprintf(
				"data:%s;base64,%s",
				sized.MediaType,
				base64.StdEncoding.EncodeToString(sized.Data),
			)},
		})
	}
	// No schema mechanism here, so the prompt carries the JSON contract.
	content = append(content, groqContent{Type: "text", Text: boardPrompt(e.now(), true)})

	return &groqRequest{
		Model:     e.model,
		MaxTokens: groqMaxTokens,
		// Extraction should be repeatable, not creative.
		Temperature:    0,
		ResponseFormat: map[string]string{"type": "json_object"},
		Messages:       []groqMessage{{Role: "user", Content: content}},
	}, nil
}

func (e *GroqExtractor) Extract(ctx context.Context, images []common.BoardImage) (*common.ExtractedSession, error) {
	payload, err := e.buildRequest(images)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("preparing the request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reading the board: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("reading the response: %w", err)
	}

	var parsed groqResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("unexpected response (HTTP %d): %s", resp.StatusCode, truncate(string(raw), 300))
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("reading the board: %s%s", parsed.Error.Message, rateLimitHint(parsed.Error.Message))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reading the board: HTTP %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("no session could be read from the image")
	}

	return decodeBoardPayload([]byte(stripCodeFence(parsed.Choices[0].Message.Content)))
}

// rateLimitHint appends the actionable fix to a token-budget rejection, which
// otherwise only tells the user to "reduce your message size" without saying
// that this app has a knob for exactly that.
func rateLimitHint(message string) string {
	m := strings.ToLower(message)
	if !strings.Contains(m, "tokens per minute") && !strings.Contains(m, "request too large") &&
		!strings.Contains(m, "rate limit") {
		return ""
	}
	return " — try fewer images at once, or lower GROQ_MAX_IMAGE_EDGE" +
		" (currently sending images at most " + itoa(DefaultGroqMaxImageEdge) + "px on the long edge by default)"
}

func itoa(v int) string { return fmt.Sprintf("%d", v) }

// stripCodeFence removes a ```json wrapper. JSON mode should prevent one, but
// vision models fall out of it often enough to be worth handling rather than
// failing an import over.
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if i := strings.Index(s, "\n"); i >= 0 {
		s = s[i+1:]
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "```"))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// MaxImageEdge reports the pixel cap applied to uploaded images, for logging.
func (e *GroqExtractor) MaxImageEdge() int { return e.maxEdge }

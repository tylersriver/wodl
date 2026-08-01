package testhelpers

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyler/wodl/internal/application/command"
	"github.com/tyler/wodl/internal/application/common"
)

// stubExtractor stands in for the vision call so the import flow can be tested
// without network access or an API key.
type stubExtractor struct {
	session *common.ExtractedSession
	err     error
	calls   int
}

func (s *stubExtractor) Extract(ctx context.Context, images []common.BoardImage) (*common.ExtractedSession, error) {
	s.calls++
	return s.session, s.err
}

// boardFromScreenshots mirrors what the two example screenshots contain: a
// lifting piece and a metcon for one day.
func boardFromScreenshots() *common.ExtractedSession {
	six, ninety, three, fifteen := 6, 90, 3, 15
	return &common.ExtractedSession{
		Name: "CrossFit - Fri, Jul 31",
		Date: time.Date(2026, 7, 31, 0, 0, 0, 0, time.Local),
		Workouts: []*common.ExtractedWorkout{
			{
				Name:            "Sumo Deadlift 6x4",
				Type:            "lifting",
				Description:     "Every 1:30 x 6 Sets\n4 Reps @ 80-85% of 5x5 Sumo Deadlift",
				Rounds:          &six,
				IntervalSeconds: &ninety,
				LiftName:        "Sumo Deadlift",
				LiftCategory:    "deadlift",
			},
			{
				Name:           "Storm Warning",
				Type:           "for_time",
				Description:    "3 Rounds\n.68/.62mi Echo Bike\n15 Hang Power Cleans\n10 Front Squats\n5 Thrusters\n\nTime Cap: 15:00\nScore = Time\n\nBarbell: 115/80",
				Rounds:         &three,
				TimeCapMinutes: &fifteen,
			},
		},
	}
}

func registerAndLogin(t *testing.T, app *TestApp) *http.Client {
	t.Helper()
	client := newClient()
	register(t, client, app.Server.URL)

	resp, err := client.PostForm(app.Server.URL+"/login", url.Values{
		"email":    {"test@example.com"},
		"password": {"password123"},
	})
	require.NoError(t, err)
	u, _ := url.Parse(app.Server.URL)
	client.Jar.SetCookies(u, resp.Cookies())
	return client
}

func userID(t *testing.T, app *TestApp) string {
	t.Helper()
	var id string
	require.NoError(t, app.DB.QueryRow("SELECT id FROM users LIMIT 1").Scan(&id))
	return id
}

// TestImport_ReusesExistingLiftAndWorkout is the behaviour that keeps Results
// from filling with near-duplicates: importing a board that names a lift and a
// workout the user already has must link to those, not make second copies.
func TestImport_ReusesExistingLiftAndWorkout(t *testing.T) {
	app := NewTestAppWithExtractor(t, &stubExtractor{session: boardFromScreenshots()})
	client := registerAndLogin(t, app)

	// Pre-existing lift and workout, deliberately cased differently to prove
	// matching is case-insensitive.
	_, err := client.PostForm(app.Server.URL+"/lifts", url.Values{
		"name": {"sumo deadlift"}, "category": {"deadlift"}, "one_rep_max": {"400"},
	})
	require.NoError(t, err)
	_, err = client.PostForm(app.Server.URL+"/workouts", url.Values{
		"name": {"STORM WARNING"}, "type": {"for_time"}, "description": {"old copy"},
	})
	require.NoError(t, err)

	var liftsBefore, workoutsBefore int
	require.NoError(t, app.DB.QueryRow("SELECT COUNT(*) FROM lifts").Scan(&liftsBefore))
	require.NoError(t, app.DB.QueryRow("SELECT COUNT(*) FROM workouts").Scan(&workoutsBefore))

	uid := userID(t, app)
	result, err := app.ImportService.Create(importCommand(t, uid, boardFromScreenshots()))
	require.NoError(t, err)

	assert.Equal(t, 1, result.ReusedLifts, "the existing lift should be reused")
	assert.Equal(t, 0, result.CreatedLifts)
	assert.Equal(t, 1, result.ReusedWorkouts, "the existing workout should be reused")
	assert.Equal(t, 1, result.CreatedWorkouts, "only the lifting piece is new")

	var liftsAfter, workoutsAfter int
	require.NoError(t, app.DB.QueryRow("SELECT COUNT(*) FROM lifts").Scan(&liftsAfter))
	require.NoError(t, app.DB.QueryRow("SELECT COUNT(*) FROM workouts").Scan(&workoutsAfter))
	assert.Equal(t, liftsBefore, liftsAfter, "no duplicate lift")
	assert.Equal(t, workoutsBefore+1, workoutsAfter, "exactly one new workout")

	// The lift's best single must survive — reusing it is the whole point.
	var orm float64
	require.NoError(t, app.DB.QueryRow(
		"SELECT one_rep_max FROM lifts WHERE LOWER(name) = 'sumo deadlift'").Scan(&orm))
	assert.Equal(t, 400.0, orm)
}

// TestImport_CreatesWhatIsMissing covers the first-import case, where nothing
// matches and everything has to be created.
func TestImport_CreatesWhatIsMissing(t *testing.T) {
	app := NewTestAppWithExtractor(t, &stubExtractor{session: boardFromScreenshots()})
	registerAndLogin(t, app)

	uid := userID(t, app)
	result, err := app.ImportService.Create(importCommand(t, uid, boardFromScreenshots()))
	require.NoError(t, err)

	assert.Equal(t, 1, result.CreatedLifts)
	assert.Equal(t, 2, result.CreatedWorkouts)
	assert.Equal(t, 0, result.ReusedLifts)
	assert.Equal(t, 0, result.ReusedWorkouts)

	// Board wording is preserved rather than reinterpreted.
	var description string
	require.NoError(t, app.DB.QueryRow(
		"SELECT description FROM workouts WHERE name = 'Storm Warning'").Scan(&description))
	assert.Contains(t, description, "Barbell: 115/80")
	assert.Contains(t, description, "Score = Time")

	// The session is dated to the board, and holds both pieces in order.
	var sessionDate time.Time
	require.NoError(t, app.DB.QueryRow(
		"SELECT session_date FROM sessions WHERE id = ?", result.SessionId.String()).Scan(&sessionDate))
	assert.Equal(t, "2026-07-31", sessionDate.Format("2006-01-02"))

	var count int
	require.NoError(t, app.DB.QueryRow(
		"SELECT COUNT(*) FROM session_workouts WHERE session_id = ?", result.SessionId.String()).Scan(&count))
	assert.Equal(t, 2, count)
}

// TestImport_ExtractRendersReviewWithoutSaving is the safety property behind
// the review step: reading images must not write anything.
func TestImport_ExtractRendersReviewWithoutSaving(t *testing.T) {
	stub := &stubExtractor{session: boardFromScreenshots()}
	app := NewTestAppWithExtractor(t, stub)
	client := registerAndLogin(t, app)

	body, contentType := imageUpload(t, "board.png", "image/png")
	resp, err := client.Post(app.Server.URL+"/import/extract", contentType, body)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	html, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	page := string(html)

	assert.Equal(t, 1, stub.calls, "the extractor should be called once")
	assert.Contains(t, page, "Storm Warning")
	assert.Contains(t, page, "Sumo Deadlift")
	assert.Contains(t, page, "2026-07-31", "the board's date should pre-fill the form")
	assert.Contains(t, page, "Barbell: 115/80", "board wording should be shown for review")

	for _, table := range []string{"sessions", "workouts", "lifts"} {
		var count int
		require.NoError(t, app.DB.QueryRow("SELECT COUNT(*) FROM "+table).Scan(&count))
		assert.Zerof(t, count, "%s should be untouched until the user saves", table)
	}
}

// TestImport_DisabledWithoutExtractor confirms the feature stays inert when no
// API key is configured, rather than erroring at upload time.
func TestImport_DisabledWithoutExtractor(t *testing.T) {
	app := NewTestApp(t)
	client := registerAndLogin(t, app)

	assert.False(t, app.ImportService.Enabled())

	resp, err := client.Get(app.Server.URL + "/import")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/sessions", resp.Header.Get("Location"))

	// The entry point should not be advertised either.
	resp, err = client.Get(app.Server.URL + "/sessions")
	require.NoError(t, err)
	defer resp.Body.Close()
	html, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.NotContains(t, string(html), `href="/import"`)
}

// TestImport_RejectsNonImageUpload keeps a bad upload from spending a request.
func TestImport_RejectsNonImageUpload(t *testing.T) {
	stub := &stubExtractor{session: boardFromScreenshots()}
	app := NewTestAppWithExtractor(t, stub)
	client := registerAndLogin(t, app)

	body, contentType := imageUpload(t, "notes.txt", "text/plain")
	resp, err := client.Post(app.Server.URL+"/import/extract", contentType, body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Zero(t, stub.calls, "the extractor should not be called for a bad upload")
}

func importCommand(t *testing.T, userId string, s *common.ExtractedSession) *command.ImportSessionCommand {
	t.Helper()
	cmd := &command.ImportSessionCommand{
		Name:   s.Name,
		Date:   s.Date,
		Warmup: s.Warmup,
	}
	require.NoError(t, cmd.UserId.UnmarshalText([]byte(userId)))
	for _, w := range s.Workouts {
		cmd.Workouts = append(cmd.Workouts, command.ImportWorkout{
			Name:            w.Name,
			Type:            w.Type,
			Description:     w.Description,
			TimeCap:         w.TimeCapMinutes,
			Rounds:          w.Rounds,
			IntervalSeconds: w.IntervalSeconds,
			LiftName:        w.LiftName,
			LiftCategory:    w.LiftCategory,
		})
	}
	return cmd
}

func imageUpload(t *testing.T, filename, contentType string) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	header := make(map[string][]string)
	header["Content-Disposition"] = []string{
		`form-data; name="images"; filename="` + filename + `"`,
	}
	header["Content-Type"] = []string{contentType}
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = io.Copy(part, strings.NewReader("not-a-real-image-payload"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	return &buf, writer.FormDataContentType()
}

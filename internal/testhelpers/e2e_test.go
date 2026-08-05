package testhelpers

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func register(t *testing.T, client *http.Client, baseURL string) {
	t.Helper()
	resp, err := client.PostForm(baseURL+"/register", url.Values{
		"email":        {"test@example.com"},
		"password":     {"password123"},
		"display_name": {"Test User"},
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
}

func TestE2E_AuthFlow(t *testing.T) {
	app := NewTestApp(t)
	client := newClient()

	// Unauthenticated access redirects to login
	resp, err := client.Get(app.Server.URL + "/")
	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/login", resp.Header.Get("Location"))

	// Register
	register(t, client, app.Server.URL)

	// Login page loads
	resp, err = client.Get(app.Server.URL + "/login")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Login
	resp, err = client.PostForm(app.Server.URL+"/login", url.Values{
		"email":    {"test@example.com"},
		"password": {"password123"},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)

	// Set cookies for redirect
	u, _ := url.Parse(app.Server.URL)
	client.Jar.SetCookies(u, resp.Cookies())

	// Dashboard now accessible
	resp, err = client.Get(app.Server.URL + "/")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestE2E_LiftFlow(t *testing.T) {
	app := NewTestApp(t)
	client := newClient()

	// Register & login
	resp, err := client.PostForm(app.Server.URL+"/register", url.Values{
		"email":        {"lifter@example.com"},
		"password":     {"password123"},
		"display_name": {"Lifter"},
	})
	require.NoError(t, err)
	u, _ := url.Parse(app.Server.URL)
	client.Jar.SetCookies(u, resp.Cookies())

	// Create lift
	resp, err = client.PostForm(app.Server.URL+"/lifts", url.Values{
		"name":        {"Back Squat"},
		"category":    {"squat"},
		"one_rep_max": {"315"},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)

	// Lifts are listed on the combined results page
	resp, err = client.Get(app.Server.URL + "/results")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Get lift ID from DB
	var liftId string
	err = app.DB.QueryRow("SELECT id FROM lifts LIMIT 1").Scan(&liftId)
	require.NoError(t, err)

	// View lift detail
	resp, err = client.Get(app.Server.URL + "/lifts/" + liftId)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Log a set
	resp, err = client.PostForm(app.Server.URL+"/lifts/"+liftId+"/logs", url.Values{
		"weight": {"225"},
		"reps":   {"5"},
		"sets":   {"3"},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)

	// Verify log exists
	var logCount int
	err = app.DB.QueryRow("SELECT COUNT(*) FROM lift_logs WHERE lift_id = ?", liftId).Scan(&logCount)
	require.NoError(t, err)
	assert.Equal(t, 1, logCount)

	// Test 1RM calculator API
	resp, err = client.Get(app.Server.URL + "/api/1rm-calc?weight=200&reps=5")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestE2E_WorkoutFlow(t *testing.T) {
	app := NewTestApp(t)
	client := newClient()

	// Register
	resp, err := client.PostForm(app.Server.URL+"/register", url.Values{
		"email":        {"crossfitter@example.com"},
		"password":     {"password123"},
		"display_name": {"CrossFitter"},
	})
	require.NoError(t, err)
	u, _ := url.Parse(app.Server.URL)
	client.Jar.SetCookies(u, resp.Cookies())

	// Create workout
	resp, err = client.PostForm(app.Server.URL+"/workouts", url.Values{
		"name":        {"Fran"},
		"type":        {"for_time"},
		"description": {"21-15-9\nThrusters (95/65)\nPull-ups"},
		"time_cap":    {"600"},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)

	// Get workout ID
	var workoutId string
	err = app.DB.QueryRow("SELECT id FROM workouts LIMIT 1").Scan(&workoutId)
	require.NoError(t, err)

	// View detail
	resp, err = client.Get(app.Server.URL + "/workouts/" + workoutId)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Log result
	resp, err = client.PostForm(app.Server.URL+"/workouts/"+workoutId+"/results", url.Values{
		"score":      {"4:30"},
		"score_type": {"time"},
		"rx":         {"on"},
		"notes":      {"PR!"},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)

	// Verify result
	var resultCount int
	err = app.DB.QueryRow("SELECT COUNT(*) FROM workout_results WHERE workout_id = ?", workoutId).Scan(&resultCount)
	require.NoError(t, err)
	assert.Equal(t, 1, resultCount)
}

func TestE2E_UnauthenticatedRedirects(t *testing.T) {
	app := NewTestApp(t)
	client := newClient()

	paths := []string{"/", "/results", "/sessions"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			resp, err := client.Get(app.Server.URL + path)
			require.NoError(t, err)
			assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
			assert.Equal(t, "/login", resp.Header.Get("Location"))
		})
	}
}

func TestE2E_DuplicateEmail(t *testing.T) {
	app := NewTestApp(t)
	client := newClient()

	register(t, client, app.Server.URL)

	// Try registering again with same email
	resp, err := client.PostForm(app.Server.URL+"/register", url.Values{
		"email":        {"test@example.com"},
		"password":     {"password123"},
		"display_name": {"Another User"},
	})
	require.NoError(t, err)
	// Should render registration page with error (200, not redirect)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestE2E_InvalidLogin(t *testing.T) {
	app := NewTestApp(t)
	client := newClient()

	resp, err := client.PostForm(app.Server.URL+"/login", url.Values{
		"email":    {"nobody@example.com"},
		"password": {"wrongpassword"},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestE2E_UpdateAndDeleteLift(t *testing.T) {
	app := NewTestApp(t)
	client := newClient()

	// Register
	resp, err := client.PostForm(app.Server.URL+"/register", url.Values{
		"email":        {"updater@example.com"},
		"password":     {"password123"},
		"display_name": {"Updater"},
	})
	require.NoError(t, err)
	u, _ := url.Parse(app.Server.URL)
	client.Jar.SetCookies(u, resp.Cookies())

	// Create lift
	resp, err = client.PostForm(app.Server.URL+"/lifts", url.Values{
		"name":     {"Bench Press"},
		"category": {"bench"},
	})
	require.NoError(t, err)

	var liftId string
	err = app.DB.QueryRow("SELECT id FROM lifts LIMIT 1").Scan(&liftId)
	require.NoError(t, err)

	// Update lift via PUT (using _method override)
	resp, err = client.PostForm(app.Server.URL+"/lifts/"+liftId, url.Values{
		"_method":     {"PUT"},
		"name":        {"Incline Bench"},
		"category":    {"bench"},
		"one_rep_max": {"225"},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)

	// Verify update
	var name string
	err = app.DB.QueryRow("SELECT name FROM lifts WHERE id = ?", liftId).Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "Incline Bench", name)

	// Delete lift
	req, _ := http.NewRequest(http.MethodDelete, app.Server.URL+"/lifts/"+liftId, nil)
	resp, err = client.Do(req)
	require.NoError(t, err)

	// Verify soft delete
	var deletedAt *string
	err = app.DB.QueryRow("SELECT deleted_at FROM lifts WHERE id = ?", liftId).Scan(&deletedAt)
	require.NoError(t, err)
	assert.NotNil(t, deletedAt)
}

func TestE2E_Calc1RM_API(t *testing.T) {
	app := NewTestApp(t)
	client := newClient()

	// Register
	resp, err := client.PostForm(app.Server.URL+"/register", url.Values{
		"email":        {"calc@example.com"},
		"password":     {"password123"},
		"display_name": {"Calculator"},
	})
	require.NoError(t, err)
	u, _ := url.Parse(app.Server.URL)
	client.Jar.SetCookies(u, resp.Cookies())

	// 200 lbs x 5 reps = 233.3
	resp, err = client.Get(fmt.Sprintf("%s/api/1rm-calc?weight=200&reps=5", app.Server.URL))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "233.3", string(body))
}

// The landing page shows the day the *reader* is on, not the day the server's
// UTC clock is on. This is the bug the tz cookie fixes: from early evening in
// the Americas the two disagree, and the app was showing tomorrow's plan.
//
// The two zones straddle UTC so that whenever the test runs, at least one of
// them is on a different calendar day than the server: Kiritimati is ahead from
// 10:00 UTC and Midway is behind until 11:00. A test that only asserted against
// the server's own zone would pass on the broken code half the day.
func TestE2E_TodayIsInTheReadersTimeZone(t *testing.T) {
	zones := []string{
		"Pacific/Kiritimati", // UTC+14 — already tomorrow from 10:00 UTC
		"Pacific/Midway",     // UTC-11 — still yesterday until 11:00 UTC
	}

	for _, zone := range zones {
		t.Run(zone, func(t *testing.T) {
			app := NewTestApp(t)
			client := newClient()

			resp, err := client.PostForm(app.Server.URL+"/register", url.Values{
				"email":        {"traveller@example.com"},
				"password":     {"password123"},
				"display_name": {"Traveller"},
			})
			require.NoError(t, err)
			u, _ := url.Parse(app.Server.URL)
			client.Jar.SetCookies(u, resp.Cookies())
			client.Jar.SetCookies(u, []*http.Cookie{{Name: "tz", Value: zone, Path: "/"}})

			loc, err := time.LoadLocation(zone)
			require.NoError(t, err)
			there := time.Now().In(loc).Format("2006-01-02")

			resp, err = client.PostForm(app.Server.URL+"/sessions", url.Values{
				"date": {there},
				"name": {"Session in " + zone},
			})
			require.NoError(t, err)
			require.Equal(t, http.StatusSeeOther, resp.StatusCode)
			// Saving a session for the day you are on lands you back on it.
			assert.Equal(t, "/", resp.Header.Get("Location"))

			resp, err = client.Get(app.Server.URL + "/")
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			body, _ := io.ReadAll(resp.Body)
			assert.Contains(t, string(body), "Session in "+zone,
				"the session dated %s (today in %s) should be on the landing page", there, zone)
		})
	}
}

// Without a zone the app can only use the server's, which is what it did before
// the cookie existed — the page must still render rather than error.
func TestE2E_TodayWithoutTimeZoneCookie(t *testing.T) {
	app := NewTestApp(t)
	client := newClient()

	resp, err := client.PostForm(app.Server.URL+"/register", url.Values{
		"email":        {"zoneless@example.com"},
		"password":     {"password123"},
		"display_name": {"Zoneless"},
	})
	require.NoError(t, err)
	u, _ := url.Parse(app.Server.URL)
	client.Jar.SetCookies(u, resp.Cookies())

	resp, err = client.PostForm(app.Server.URL+"/sessions", url.Values{
		"date": {time.Now().Format("2006-01-02")},
		"name": {"Server-zone session"},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	resp, err = client.Get(app.Server.URL + "/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "Server-zone session")
}

// Each workout in a session folds away, so a long day isn't several screens of
// scrolling to reach the step you're on. It's a native <details> and starts
// open — collapsing is something you do, not a state you're greeted by.
func TestE2E_SessionCardsAreCollapsible(t *testing.T) {
	app := NewTestApp(t)
	client := newClient()

	resp, err := client.PostForm(app.Server.URL+"/register", url.Values{
		"email":        {"folder@example.com"},
		"password":     {"password123"},
		"display_name": {"Folder"},
	})
	require.NoError(t, err)
	u, _ := url.Parse(app.Server.URL)
	client.Jar.SetCookies(u, resp.Cookies())

	resp, err = client.PostForm(app.Server.URL+"/workouts", url.Values{
		"name":        {"Fran"},
		"type":        {"for_time"},
		"description": {"21-15-9 thrusters and pull-ups"},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	var workoutId string
	require.NoError(t, app.DB.QueryRow("SELECT id FROM workouts LIMIT 1").Scan(&workoutId))

	resp, err = client.PostForm(app.Server.URL+"/sessions", url.Values{
		"date":        {time.Now().Format("2006-01-02")},
		"name":        {"Fold me"},
		"warmup":      {"500m row"},
		"workout_ids": {workoutId},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	resp, err = client.Get(app.Server.URL + "/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	page := string(body)

	assert.Contains(t, page, `data-card-key="workout-`+workoutId+`"`)
	assert.Contains(t, page, `data-card-key="warmup-`)
	assert.Contains(t, page, "<details", "cards fold without JavaScript")
	// The body is still in the markup, so a collapsed card costs nothing to
	// open and the page is still searchable and printable.
	assert.Contains(t, page, "21-15-9 thrusters and pull-ups")

	// The session's own page shows the same cards, from the same partial.
	var sessionId string
	require.NoError(t, app.DB.QueryRow("SELECT id FROM sessions LIMIT 1").Scan(&sessionId))
	resp, err = client.Get(app.Server.URL + "/sessions/" + sessionId)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ = io.ReadAll(resp.Body)
	assert.Contains(t, string(body), `data-card-key="workout-`+workoutId+`"`)
	assert.Contains(t, string(body), `data-card-key="warmup-`+sessionId+`"`)
}

// loggedMarks counts the "done" ticks on a sessions page. The mark is only ever
// rendered for a day something was logged on, so its count is the assertion.
func loggedMarks(page string) int {
	return strings.Count(page, `aria-label="Logged"`)
}

func getPage(t *testing.T, client *http.Client, url string) string {
	t.Helper()
	resp, err := client.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

// The week and month views mark a day whose plan was at least started. Nothing
// records that a session was performed — it is a plan — so the mark is earned by
// logging something the day prescribed, on that day.
func TestE2E_SessionViewsMarkLoggedDays(t *testing.T) {
	app := NewTestApp(t)
	client := newClient()

	resp, err := client.PostForm(app.Server.URL+"/register", url.Values{
		"email":        {"marker@example.com"},
		"password":     {"password123"},
		"display_name": {"Marker"},
	})
	require.NoError(t, err)
	u, _ := url.Parse(app.Server.URL)
	client.Jar.SetCookies(u, resp.Cookies())

	resp, err = client.PostForm(app.Server.URL+"/workouts", url.Values{
		"name":        {"Fran"},
		"type":        {"for_time"},
		"description": {"21-15-9 thrusters and pull-ups"},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	var workoutId string
	require.NoError(t, app.DB.QueryRow("SELECT id FROM workouts LIMIT 1").Scan(&workoutId))

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	// Yesterday's plan, which was never done.
	resp, err = client.PostForm(app.Server.URL+"/sessions", url.Values{
		"date":        {yesterday},
		"name":        {"Yesterday"},
		"workout_ids": {workoutId},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	// Today's plan, prescribing the same workout.
	resp, err = client.PostForm(app.Server.URL+"/sessions", url.Values{
		"date":        {today},
		"name":        {"Today"},
		"workout_ids": {workoutId},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	// A planned day is not a done one.
	assert.Zero(t, loggedMarks(getPage(t, client, app.Server.URL+"/sessions?view=week")),
		"nothing is logged yet, so no day is marked")
	assert.Zero(t, loggedMarks(getPage(t, client, app.Server.URL+"/sessions?view=calendar")),
		"nothing is logged yet, so no day is marked")

	resp, err = client.PostForm(app.Server.URL+"/workouts/"+workoutId+"/results", url.Values{
		"score":      {"3:21"},
		"score_type": {"time"},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	// Exactly one: the score was logged today, and a repeated benchmark must not
	// backdate itself onto every session that ever prescribed it.
	assert.Equal(t, 1, loggedMarks(getPage(t, client, app.Server.URL+"/sessions?view=week")),
		"only the day the score was logged on is marked")
	assert.Equal(t, 1, loggedMarks(getPage(t, client, app.Server.URL+"/sessions?view=calendar")),
		"only the day the score was logged on is marked")
}

// Lifting steps never produce a workout result: sets are logged against the lift
// itself. The mark has to follow them there or a squat day never reads as done.
func TestE2E_SessionViewsMarkLoggedLiftSets(t *testing.T) {
	app := NewTestApp(t)
	client := newClient()

	resp, err := client.PostForm(app.Server.URL+"/register", url.Values{
		"email":        {"lift-marker@example.com"},
		"password":     {"password123"},
		"display_name": {"Lift Marker"},
	})
	require.NoError(t, err)
	u, _ := url.Parse(app.Server.URL)
	client.Jar.SetCookies(u, resp.Cookies())

	resp, err = client.PostForm(app.Server.URL+"/lifts", url.Values{
		"name":        {"Back Squat"},
		"category":    {"squat"},
		"one_rep_max": {"315"},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	var liftId string
	require.NoError(t, app.DB.QueryRow("SELECT id FROM lifts LIMIT 1").Scan(&liftId))

	today := time.Now().Format("2006-01-02")

	resp, err = client.PostForm(app.Server.URL+"/workouts", url.Values{
		"type":        {"lifting"},
		"lift_id":     {liftId},
		"date":        {today},
		"description": {"5x3 @ 80%"},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	var workoutId string
	require.NoError(t, app.DB.QueryRow("SELECT id FROM workouts LIMIT 1").Scan(&workoutId))

	resp, err = client.PostForm(app.Server.URL+"/sessions", url.Values{
		"date":        {today},
		"name":        {"Squat day"},
		"workout_ids": {workoutId},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	assert.Zero(t, loggedMarks(getPage(t, client, app.Server.URL+"/sessions?view=week")),
		"the squat is only prescribed so far")

	resp, err = client.PostForm(app.Server.URL+"/lifts/"+liftId+"/logs", url.Values{
		"weight": {"255"},
		"reps":   {"3"},
		"sets":   {"5"},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	assert.Equal(t, 1, loggedMarks(getPage(t, client, app.Server.URL+"/sessions?view=week")),
		"sets logged against the lift mark the day that prescribed it")
	assert.Equal(t, 1, loggedMarks(getPage(t, client, app.Server.URL+"/sessions?view=calendar")),
		"sets logged against the lift mark the day that prescribed it")
}

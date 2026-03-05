package testhelpers

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"

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

	// List lifts
	resp, err = client.Get(app.Server.URL + "/lifts")
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

	paths := []string{"/", "/lifts", "/workouts"}
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

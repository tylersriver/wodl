package handlers

import (
	"net/http"
	"testing"
	"time"
)

func requestWithTZ(tz string) *http.Request {
	r, _ := http.NewRequest(http.MethodGet, "/", nil)
	if tz != "" {
		r.AddCookie(&http.Cookie{Name: tzCookieName, Value: tz})
	}
	return r
}

func TestRequestLocation_UsesCookie(t *testing.T) {
	loc := requestLocation(requestWithTZ("America/Denver"))
	if loc.String() != "America/Denver" {
		t.Fatalf("got %q, want America/Denver", loc)
	}
}

// Cookie values arrive verbatim, so a client that percent-encoded the zone name
// must not silently degrade to the server's own zone — that failure is
// indistinguishable from the bug being fixed.
func TestRequestLocation_AcceptsAnEncodedName(t *testing.T) {
	loc := requestLocation(requestWithTZ("America%2FDenver"))
	if loc.String() != "America/Denver" {
		t.Fatalf("got %q, want America/Denver", loc)
	}
}

// A missing or unusable cookie must not fail the request — it degrades to the
// server's own zone, which is what the app did before the cookie existed.
func TestRequestLocation_FallsBackToServerZone(t *testing.T) {
	cases := []struct{ name, tz string }{
		{"absent", ""},
		{"unknown zone", "Mars/Olympus_Mons"},
		{"path escape", "../../etc/passwd"},
		{"nonsense", "not a timezone at all"},
		{"offset rather than a name", "UTC-7"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if loc := requestLocation(requestWithTZ(c.tz)); loc != time.Local {
				t.Fatalf("got %q, want the server zone", loc)
			}
		})
	}
}

// The bug this whole file exists for: at 19:35 on a Sunday in Denver it is
// already Monday in UTC, and the landing page was showing Monday's plan.
func TestTodayIn_IsTheReadersDayNotTheServers(t *testing.T) {
	denver, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatalf("loading zone: %v", err)
	}

	// A moment that is Sunday evening in Denver and Monday morning in UTC.
	sundayEvening := time.Date(2026, 8, 3, 1, 35, 0, 0, time.UTC)
	got := civilDate(sundayEvening.In(denver))

	want := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
	if got.Weekday() != time.Sunday {
		t.Fatalf("got %s, want Sunday", got.Weekday())
	}
}

// Stored session dates are civil: the same day for everyone who reads them, and
// anchored where the rows already in the database are.
func TestCivilDate_AnchorsAtMidnightUTC(t *testing.T) {
	kiritimati, err := time.LoadLocation("Pacific/Kiritimati") // UTC+14
	if err != nil {
		t.Fatalf("loading zone: %v", err)
	}

	parsed, err := parseCivilDate("2026-08-02")
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	want := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	if !parsed.Equal(want) {
		t.Fatalf("got %s, want %s", parsed, want)
	}

	// Reading it back must not shift the day, whoever is reading.
	if got := formatCivil(parsed.In(kiritimati)); got != "2026-08-02" {
		t.Fatalf("got %s, want 2026-08-02", got)
	}
}

func TestParseSessionDate_FallsBackToToday(t *testing.T) {
	today := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	for _, v := range []string{"", "not-a-date", "2026-13-45"} {
		if got := parseSessionDate(v, today); !got.Equal(today) {
			t.Fatalf("parseSessionDate(%q) = %s, want today", v, got)
		}
	}
	if got := parseSessionDate("2026-09-01", today); !got.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("got %s, want 2026-09-01", got)
	}
}

// A session saved for the day the user is on lands them back on it.
func TestSessionRedirectTarget(t *testing.T) {
	today := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	if got := sessionRedirectTarget(today, today); got != "/" {
		t.Fatalf("got %q, want /", got)
	}
	if got := sessionRedirectTarget(today.AddDate(0, 0, 1), today); got != "/sessions" {
		t.Fatalf("got %q, want /sessions", got)
	}
}

func TestParseWeekParam_StartsOnSunday(t *testing.T) {
	today := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC) // a Wednesday

	got, err := parseWeekParam("", today)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	want := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}

	got, err = parseWeekParam("2026-08-14", today)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	if want := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestParseMonthParam(t *testing.T) {
	today := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)

	got, err := parseMonthParam("", today)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	if want := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}

	if _, err := parseMonthParam("nope", today); err == nil {
		t.Fatal("want an error for an unparseable month")
	}
}

// The day a user picks has to come back out as the day they picked, read in
// their own zone. March 14 also lands on a DST change in the American zones,
// where a day is 23 hours long.
func TestInstantOnDay_RoundTripsThePickedDay(t *testing.T) {
	zones := []string{"America/Denver", "America/Santiago", "Asia/Tokyo", "Pacific/Auckland", "UTC"}
	day := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)

	for _, name := range zones {
		t.Run(name, func(t *testing.T) {
			loc, err := time.LoadLocation(name)
			if err != nil {
				t.Fatalf("loading zone: %v", err)
			}
			if got := formatCivil(dayIn(instantOnDay(day, loc), loc)); got != "2026-03-14" {
				t.Fatalf("got %s, want 2026-03-14", got)
			}
		})
	}
}

// The tz cookie is best-effort: a later request can arrive without one and fall
// back to the server's zone, which is UTC. A score must not slide onto the day
// before when that happens, which is what anchoring at midday rather than
// midnight buys. Nothing can cover every zone — the offsets span 26 hours — so
// this is the UTC-11..UTC+12 band, everyone but the Pacific's far east.
func TestInstantOnDay_SurvivesAZonelessRead(t *testing.T) {
	zones := []string{"Pacific/Pago_Pago", "America/Denver", "Europe/Berlin", "Asia/Tokyo", "UTC"}
	day := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)

	for _, name := range zones {
		t.Run(name, func(t *testing.T) {
			loc, err := time.LoadLocation(name)
			if err != nil {
				t.Fatalf("loading zone: %v", err)
			}
			if got := formatCivil(dayIn(instantOnDay(day, loc), time.UTC)); got != "2026-03-14" {
				t.Fatalf("read back in UTC: got %s, want 2026-03-14", got)
			}
		})
	}
}

// Logging as you finish is the ordinary case, and it should still record the
// moment it happened rather than a flat midday — history reads in the order the
// work was done.
func TestInstantOnDay_KeepsTheClockForToday(t *testing.T) {
	denver, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatalf("loading zone: %v", err)
	}

	at := instantOnDay(todayIn(denver), denver)
	if d := time.Since(at); d < 0 || d > time.Minute {
		t.Fatalf("today logged %v ago, want roughly now", d)
	}
}

// A nil zone is what a request without a usable tz cookie hands down; it must
// resolve rather than panic.
func TestInstantOnDay_ToleratesNoZone(t *testing.T) {
	day := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
	if got := formatCivil(dayIn(instantOnDay(day, nil), time.Local)); got != "2026-03-14" {
		t.Fatalf("got %s, want 2026-03-14", got)
	}
}

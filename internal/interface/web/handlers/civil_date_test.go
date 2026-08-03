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

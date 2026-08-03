package handlers

import (
	"net/http"
	"net/url"
	"time"

	// requestLocation resolves IANA zone names, and the Alpine image the app
	// ships in carries no zoneinfo — without the embedded copy every lookup
	// fails there and every user silently falls back to UTC, which is the bug
	// this file exists to fix. It lives next to the lookup rather than in main
	// so the tests below load zones the same way production does.
	_ "time/tzdata"
)

// tzCookieName carries the browser's IANA time zone name, e.g. "America/Denver".
//
// The server has no other way to know it. Requests arrive without a zone and the
// container's clock is UTC, so "today" used to mean today-in-UTC — which from
// early evening onwards in the Americas is already tomorrow, and the landing
// page would show Monday's plan on Sunday night. layout.html sets this cookie
// from the browser and re-requests the page once, so even a first visit renders
// in the reader's zone.
const tzCookieName = "tz"

// requestLocation is the zone this request's calendar days are measured in.
//
// Anything unusable — no cookie, a hand-edited one, a zone this build's tzdata
// doesn't know — falls back to the server's own zone. That is exactly the
// behaviour the app had before the cookie existed, so the failure mode is the
// old bug rather than an error page.
func requestLocation(r *http.Request) *time.Location {
	c, err := r.Cookie(tzCookieName)
	if err != nil || c.Value == "" {
		return time.Local
	}
	if loc, err := time.LoadLocation(c.Value); err == nil {
		return loc
	}
	// Go hands back cookie values exactly as they arrived, so a client that
	// percent-encoded the name sends "America%2FDenver" — not a zone anything
	// can look up, and the failure is silent and looks exactly like the bug
	// this file fixes. Cheaper to accept both spellings than to depend on
	// every caller writing the cookie the one right way.
	if unescaped, err := url.QueryUnescape(c.Value); err == nil && unescaped != c.Value {
		if loc, err := time.LoadLocation(unescaped); err == nil {
			return loc
		}
	}
	return time.Local
}

// civilDate reduces an instant to the calendar day it fell on, anchored at
// midnight UTC.
//
// A session date is a civil date, not an instant: "the plan for Sunday" is the
// plan for Sunday wherever it is read from, and must not slide into Saturday
// because the reader is west of the server. Anchoring on UTC is what makes that
// stable, and it is also what every row already in the database holds — they
// were written as local midnight back when the server's own zone was the only
// one in play, and that zone is UTC.
func civilDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// todayIn is the user's current calendar day, as a civil date.
func todayIn(loc *time.Location) time.Time {
	return civilDate(time.Now().In(loc))
}

// parseCivilDate reads a YYYY-MM-DD form value or query parameter. Such a value
// never carries a zone, so it is a civil date by construction.
func parseCivilDate(v string) (time.Time, error) {
	return time.Parse(sessionDateLayout, v)
}

// formatCivil renders a stored civil date. Reading it back in UTC returns the
// day it was written as; formatting it in the reader's zone would shift it.
func formatCivil(t time.Time) string {
	return t.UTC().Format(sessionDateLayout)
}

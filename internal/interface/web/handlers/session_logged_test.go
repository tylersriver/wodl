package handlers

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tyler/wodl/internal/application/common"
)

// The other half of the zone story: a session date is a civil date, but a result
// is an instant, and it has to land on the day the lifter thinks they did it.
// At 7:35pm on a Sunday in Denver it is already Monday in UTC, and the tick
// belongs on Sunday.
func TestDayIn_IsTheDayTheReaderLoggedIt(t *testing.T) {
	denver, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatalf("loading zone: %v", err)
	}

	sundayEvening := time.Date(2026, 8, 3, 1, 35, 0, 0, time.UTC)

	if got := formatCivil(dayIn(sundayEvening, denver)); got != "2026-08-02" {
		t.Fatalf("got %s, want 2026-08-02", got)
	}
	if got := formatCivil(dayIn(sundayEvening, time.UTC)); got != "2026-08-03" {
		t.Fatalf("got %s, want 2026-08-03", got)
	}
}

// The bounds handed to the repositories are instants, so the window has to open
// where the reader's day does — six hours later than UTC's, in Denver's case.
func TestStartOfDayIn_OpensTheWindowInTheReadersZone(t *testing.T) {
	denver, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatalf("loading zone: %v", err)
	}

	day, err := parseCivilDate("2026-08-02")
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	got := startOfDayIn(day, denver)
	if want := time.Date(2026, 8, 2, 6, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("got %s, want %s", got.UTC(), want)
	}
	if got := startOfDayIn(day, time.UTC); !got.Equal(day) {
		t.Fatalf("got %s, want %s", got, day)
	}
}

func TestLoggedIndex_Done(t *testing.T) {
	monday, tuesday := "2026-08-03", "2026-08-04"
	fran, squat := uuid.New(), uuid.New()
	backSquat := uuid.New()

	idx := &loggedIndex{
		workouts: map[string]map[uuid.UUID]bool{monday: {fran: true}},
		lifts:    map[string]map[uuid.UUID]bool{monday: {backSquat: true}},
	}

	session := func(date string, workouts ...*common.WorkoutResult) *common.SessionResult {
		d, err := parseCivilDate(date)
		if err != nil {
			t.Fatalf("parsing: %v", err)
		}
		return &common.SessionResult{Date: d, Workouts: workouts}
	}

	franWorkout := &common.WorkoutResult{Id: fran}
	squatWorkout := &common.WorkoutResult{Id: squat, LiftId: &backSquat}
	unloggedWorkout := &common.WorkoutResult{Id: uuid.New()}

	cases := []struct {
		name string
		in   *common.SessionResult
		want bool
	}{
		{"a score logged on the day", session(monday, unloggedWorkout, franWorkout), true},
		{"sets logged against the lift a step prescribes", session(monday, squatWorkout), true},
		{"nothing logged", session(monday, unloggedWorkout), false},
		// The same benchmark on another day must not inherit Monday's tick, or
		// every session that ever prescribed Fran would read as done.
		{"the same workout on another day", session(tuesday, franWorkout), false},
		{"the same lift on another day", session(tuesday, squatWorkout), false},
		{"an empty session", session(monday), false},
		{"no session at all", nil, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := idx.done(c.in); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}

	if !idx.anyDone([]*common.SessionResult{session(tuesday, franWorkout), session(monday, franWorkout)}) {
		t.Fatal("a day is done when any of its sessions is")
	}
	if idx.anyDone(nil) {
		t.Fatal("a day with no sessions is not done")
	}
}

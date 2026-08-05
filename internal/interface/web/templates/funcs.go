package templates

import (
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/tyler/wodl/internal/interface/web/static"
)

// Must parses the whole template set with the helpers every page relies on.
//
// It lives here rather than at each call site because there are two — the server
// and the end-to-end test harness — and when they drifted apart the tests booted
// a different template set than production did. A helper added for a page would
// then panic only in whichever one had been forgotten.
func Must() *template.Template {
	return template.Must(template.New("").Funcs(funcMap()).ParseFS(FS, "*.html"))
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"deref": func(f *float64) float64 {
			if f == nil {
				return 0
			}
			return *f
		},
		"derefInt": func(i *int) int {
			if i == nil {
				return 0
			}
			return *i
		},
		"inc":   func(i int) int { return i + 1 },
		"dict":  dict,
		"when":  when,
		"label": label,
		// Content-addressed URLs for cached assets, so a new build's markup can
		// never be paired with the previous build's stylesheet.
		"asset": static.AssetURL,
	}
}

// label renders one of the domain's enums as something to read.
//
// They are stored snake_case — "for_time", "rounds_and_reps" — which passed
// unnoticed inside a lowercase badge and does not once the same value is set as
// a tracked-out capital above a heading. Only the rendering changes: the value
// posted back is still the enum, so nothing downstream has to know.
//
// Takes any because the callers pass typed strings (entities.WorkoutType,
// entities.LiftCategory) as often as plain ones.
func label(v any) string {
	return strings.ReplaceAll(fmt.Sprintf("%v", v), "_", " ")
}

// when formats an instant in the reader's zone, which the handler passes down
// as `Loc`.
//
// Logging a set at 7pm in Denver records an instant that is already the next
// day in UTC, and the server's clock is UTC — so `.LoggedAt.Format` alone dates
// half the evening's work to tomorrow. A nil zone means the browser never told
// us one; fall back to the server's, which is what the whole app did before.
func when(t time.Time, loc *time.Location, layout string) string {
	if loc == nil {
		loc = time.Local
	}
	return t.In(loc).Format(layout)
}

// dict builds a map inline, so a partial can be given more than the one value a
// range or template action passes down.
func dict(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict: odd number of args")
	}
	m := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict: key must be string, got %T", values[i])
		}
		m[key] = values[i+1]
	}
	return m, nil
}

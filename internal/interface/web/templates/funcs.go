package templates

import (
	"fmt"
	"html/template"

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
		"inc":  func(i int) int { return i + 1 },
		"dict": dict,
		// Content-addressed URLs for cached assets, so a new build's markup can
		// never be paired with the previous build's stylesheet.
		"asset": static.AssetURL,
	}
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

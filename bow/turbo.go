package bow

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

type StreamAction string

const (
	ActionAppend  StreamAction = "append"
	ActionPrepend              = "prepend"
	ActionReplace              = "replace"
	ActionUpdate               = "update"
	ActionRemove               = "remove"
	ActionBefore               = "before"
	ActionAfter                = "after"

	streamMime string = "text/vnd.turbo-stream.html"
)

// optimizeTurboFrame is a middleware that optimizes so that
// the layout is stripped from the response for turbo frame requests.
func optimizeTurboFrame(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Turbo-Frame") != "" {
			r = StripLayout(r)
		}
		next.ServeHTTP(w, r)
	})
}

// AcceptsStream returns true if the request has got a Accept header saying
// that it accepts turbo streams in response.
func AcceptsStream(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, streamMime)
}

// RenderStream renders a partial view and wraps it in a turbo stream tag.
// It also sets the appropriate Content-Type header on the response.
func (views *Views) RenderStream(action StreamAction, target string, w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	w.Header().Set("Content-Type", streamMime)

	if data == nil {
		data = make(map[string]interface{})
	}

	var buf bytes.Buffer

	if action != ActionRemove {
		partial := views.partials.Lookup(name)
		if partial == nil {
			ServerError(w, fmt.Errorf("partial %s not found", name))
			return
		}

		if err := renderBuffered(&buf, views.partials, name, data); err != nil {
			ServerError(w, err)
			return
		}
	}

	wrapper := `<turbo-stream action="{{ .Action }}" target="{{ .Target }}">
  <template>
    {{ .Content }}
  </template>
</turbo-stream>`

	tmpl := template.Must(template.New("stream").Parse(wrapper))

	stream := struct {
		Action  StreamAction
		Target  string
		Content template.HTML
	}{action, target, template.HTML(buf.String())}

	if err := renderBuffered(w, tmpl, "stream", stream); err != nil {
		ServerError(w, err)
	}
}

package bow

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/justinas/nosurf"
)

type contextKey int

const (
	contextKeyLayout contextKey = iota
	contextKeyStripLayout

	partialPrefix = "_"
	layoutsFolder = "layouts"
)

// InjectFunc represents a func that can inject automatic
// data at the rendering of a view.
type InjectFunc func(*http.Request, map[string]interface{})

// Views is an engine that will render views from templates.
type Views struct {
	pages      map[string]*template.Template
	partials   map[string]*template.Template
	injectData InjectFunc
}

// defaultFuncs contains the default functions
// that will be added to templates.
var defaultFuncs = template.FuncMap{
	"partial": func() template.HTML { return "" }, // will be overidden at rendering
	"map":     mapFunc,
	"safe":    safe,
}

// defaultInjectData is an InjectFunc that will be called to automatically
// inject default data at the rendering of templates.
var defaultInjectData = func(r *http.Request, data map[string]interface{}) {
	data["CSRFToken"] = nosurf.Token(r)
}

// Parse walks a filesystem from the root folder to discover and parse
// html files into views. Files starting with an underscore are partial views.
// Files in the layouts folder not starting with underscore are layouts. The rest of
// html files are full page views. The funcs parameter is a list of functions that is
// attached to views.
//
// Views, layouts and partials will be referred to with their path, but without the
// root folder, and without the file extension.
//
// Layouts will be referred to without the layouts folder neither.
//
// Partials files are named with a leading underscore to distinguish them from regular views,
// but will be referred to without the underscore.
//
// injectData is a function that will be called when rendering page views so that data can be
// automatically injected.
func (views *Views) Parse(fsys fs.FS, root string, funcs template.FuncMap, injectData InjectFunc) error {
	views.pages = make(map[string]*template.Template)
	views.partials = make(map[string]*template.Template)
	views.injectData = injectData

	if funcs == nil {
		funcs = make(template.FuncMap)
	}

	// include default funcs
	for k, v := range defaultFuncs {
		funcs[k] = v
	}

	var pages, partials, layouts []string

	err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || filepath.Ext(path) != ".html" {
			return nil
		}

		dirs := strings.Split(filepath.Dir(path), string(os.PathSeparator))

		switch {
		case filepath.Base(path)[0:1] == partialPrefix:
			partials = append(partials, path)
		case len(dirs) > 1 && dirs[1] == layoutsFolder:
			layouts = append(layouts, path)
		default:
			pages = append(pages, path)
		}

		return nil
	})
	if err != nil {
		return err
	}

	for _, page := range pages {
		tmpl, err := parseTemplate(fsys, funcs, page, layouts)
		if err != nil {
			return err
		}

		views.pages[templateName(page)] = tmpl
	}

	for _, partial := range partials {
		tmpl, err := parseTemplate(fsys, funcs, partial, nil)
		if err != nil {
			return err
		}

		views.partials[templateName(partial)] = tmpl
	}

	return nil
}

// Render renders a given view or partial.
// For page views, the layout can be set using the WithLayout function or using the ApplyLayout middleware.
// If no layout is defined, the "base" layout will be chosen. Partial views are rendered without any layout.
func (views *Views) Render(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html")

	partial, ok := views.partials[name]
	if ok {
		if err := views.render(w, r, partial, "main", data); err != nil {
			ServerError(w, err)
		}
		return
	}

	view, ok := views.pages[name]
	if !ok {
		ServerError(w, fmt.Errorf("view %s not found", name))
		return
	}

	layout, ok := r.Context().Value(contextKeyLayout).(string)
	if ok {
		layout = filepath.Join(layoutsFolder, layout)
	} else {
		layout = filepath.Join(layoutsFolder, "base")
	}

	if view.Lookup(layout) == nil {
		ServerError(w, fmt.Errorf("layout %s not found", layout))
		return
	}

	skipLayout, _ := r.Context().Value(contextKeyStripLayout).(bool)
	if skipLayout {
		layout = "main"
	}

	if err := views.render(w, r, view, layout, data); err != nil {
		ServerError(w, err)
	}
}

// render renders the given template and inject default data and dynamic funcs.
func (views *Views) render(w io.Writer, r *http.Request, tmpl *template.Template, name string, data map[string]interface{}) error {
	funcs := template.FuncMap{
		"partial": func(name string, data map[string]interface{}) (template.HTML, error) {
			partial, ok := views.partials[name]
			if !ok {
				return "", fmt.Errorf("partial %s not found", name)
			}

			var buf bytes.Buffer
			if err := views.render(&buf, r, partial, "main", data); err != nil {
				return "", err
			}
			return template.HTML(buf.String()), nil
		},
	}

	if data == nil {
		data = make(map[string]interface{})
	}

	tmpl, err := tmpl.Clone()
	if err != nil {
		return err
	}

	tmpl.Funcs(funcs)

	if defaultInjectData != nil {
		defaultInjectData(r, data)
	}

	if views.injectData != nil {
		views.injectData(r, data)
	}

	return renderBuffered(w, tmpl, name, data)
}

// renderBuffered renders the given template to a buffer, and then to the writer.
// This way, if there is a problem during the rendering, an error can be returned.
func renderBuffered(w io.Writer, tmpl *template.Template, name string, data interface{}) error {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return err
	}

	if _, err := buf.WriteTo(w); err != nil {
		return err
	}

	return nil
}

// templateName returns a template name from a path.
// It removes the extension, removes the leading "_" from partials
// and trims the root directory.
func templateName(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))

	if base[0:1] == partialPrefix {
		base = base[1:]
	}

	dirs := strings.Split(filepath.Dir(path), string(os.PathSeparator))
	dir := filepath.Join(dirs[1:]...)

	return filepath.Join(dir, base)
}

// parseTemplate creates a new template from the given path and parses the main and
// associated templates from the given filesystem. It also attached funcs.
func parseTemplate(fsys fs.FS, funcs template.FuncMap, main string, associated []string) (*template.Template, error) {
	tmpl := template.New("main").Funcs(funcs)

	if main != "" {
		b, err := fs.ReadFile(fsys, main)
		if err != nil {
			return nil, err
		}

		_, err = tmpl.Parse(string(b))
		if err != nil {
			return nil, err
		}
	}

	for _, path := range associated {
		b, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, err
		}

		tmpl.New(templateName(path)).Parse(string(b))
	}

	return tmpl, nil
}

// StripLayout returns a shallow copy of the request but
// with the information that the layout should be stripped.
func StripLayout(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), contextKeyStripLayout, true)
	return r.WithContext(ctx)
}

// WithLayout returns a shallow copy of the request but
// with the information of the layout to apply.
func WithLayout(r *http.Request, layout string) *http.Request {
	ctx := context.WithValue(r.Context(), contextKeyLayout, layout)
	return r.WithContext(ctx)
}

// ApplyLayout is a middleware that applies a specific layout for the rendering of the view.
// It returns a function which has the correct signature to be used with alice, but it can
// also be used without.
//
// https://pkg.go.dev/github.com/justinas/alice#Constructor
func ApplyLayout(layout string) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, WithLayout(r, layout))
		})
	}
}

// mapFunc is a function that is injected to templates to allow to easily pass data to partials.
func mapFunc(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 {
		return nil, errors.New("invalid number of argument in map call")
	}

	data := make(map[string]interface{})

	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, errors.New("map keys must be strings")
		}
		data[key] = values[i+1]
	}

	return data, nil
}

// safe returns a verbatim unescaped HTML from a string
func safe(s string) template.HTML {
	return template.HTML(s)
}

package main

import "net/http"

// templateData contains all kinds of objects
// that can be returned in a template.
type templateData struct {
	Flash string
	Form  *Form

	AssistText map[int]string
	Event      *Event
	Events     []*Event
	Guest      *Guest
	Guests     []*Guest
	Statuses   []*Status
}

// The addDefaultDat automatically injects data that are common to all pages.
func (app *application) addDefaultData(r *http.Request, data interface{}) interface{} {
	td := data.(*templateData)
	if td == nil {
		td = &templateData{}
	}
	td.Flash = app.session.PopString(r, "flash")
	return td
}

// The setBoosted middleware updates the current layout if it is an
// htmx boosted request to only return the content of the body instead
// of the full html page.
func (app *application) setBoosted(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("HX-Boosted") == "true" {
			r = WithLayout(r, "boosted")
		}
		next.ServeHTTP(w, r)
	})
}

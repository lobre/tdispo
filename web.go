package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/justinas/nosurf"
	"github.com/lobre/tdispo/webapp"
)

type contextKey int

const (
	contextKeyCurrentGuest contextKey = iota
)

// templateData contains all kinds of objects
// that can be returned in a template.
type templateData struct {
	Flash        string
	Form         *webapp.Form
	CSRFToken    string
	IsAdmin      bool
	CurrentGuest *Guest

	AssistText           map[int]string
	Event                *Event
	Events               []*Event
	Guest                *Guest
	Guests               []*Guest
	Statuses             []*Status
	CurrentParticipation *Participation
}

// addDefaultData automatically injects data that are common to all pages.
func (app *application) addDefaultData(r *http.Request, data interface{}) interface{} {
	td := data.(*templateData)
	if td == nil {
		td = &templateData{}
	}
	td.CSRFToken = nosurf.Token(r)
	td.Flash = app.session.PopString(r, "flash")
	td.CurrentGuest = currentGuest(r)
	td.IsAdmin = app.isAdmin(r)
	return td
}

// recognizeGuest is a middleware that checks if a guest exists in the session,
// then verifies it is a valid guest. If so, it adds this info to the
// request context.
func (app *application) recognizeGuest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ok := app.session.Exists(r, "guest"); !ok {
			next.ServeHTTP(w, r)
			return
		}

		guest, err := app.guestService.FindGuestByID(r.Context(), app.session.GetInt(r, "guest"))
		if errors.Is(err, ErrNoRecord) {
			app.session.Remove(r, "guest")
		} else if err != nil {
			app.ServerError(w, err)
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyCurrentGuest, guest)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireRecognition is a middleware that redirects the user to the /whoareyou
// page if he isn’t recognized.
func requireRecognition(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if currentGuest(r) == nil {
			w.Header().Set("HX-Redirect", "/")
			http.Redirect(w, r, "/whoareyou", http.StatusSeeOther)
			return
		}

		w.Header().Add("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// currentGuest returns the currently recognized guest.
// If not recognized, it returns nil.
func currentGuest(r *http.Request) *Guest {
	guest, ok := r.Context().Value(contextKeyCurrentGuest).(*Guest)
	if !ok {
		return nil
	}
	return guest
}

// isAdmin returns true if the current user is connected
// as admin, otherwise false.
func (app *application) isAdmin(r *http.Request) bool {
	return app.session.GetBool(r, "isAdmin")
}

// requireAdmin is a middleware that redirects the user to the homepage
// page if he is not admin.
func (app *application) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !app.isAdmin(r) {
			http.NotFound(w, r)
			return
		}

		w.Header().Add("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

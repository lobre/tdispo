package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/justinas/nosurf"
	"github.com/lobre/tdispo/bow"
)

type contextKey int

const contextKeyCurrentGuest contextKey = iota

// addDefaultData automatically injects data that are common to all pages.
func (app *application) addDefaultData(r *http.Request, data map[string]interface{}) {
	data["Lang"] = app.config.lang
	data["CSRFToken"] = nosurf.Token(r)
	data["Flash"] = app.session.PopString(r, "flash")
	data["CurrentGuest"] = currentGuest(r)
	data["IsAdmin"] = app.isAdmin(r)
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
			bow.ServerError(w, err)
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

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
)

// data contains all kinds of objects
// that can be returned in a template.
type data struct {
	Errors     []string
	Statuses   []*Status
	StatusesJS template.JS
	Event      *Event
	Events     []*Event
	Guest      *Guest
	Guests     []*Guest
}

type envelope map[string]interface{}

// logRequest is a middleware that will log request to the application logger.
func (app *application) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.logger.Printf("%s - %s %s %s", r.RemoteAddr, r.Proto, r.Method, r.URL.RequestURI())
		next.ServeHTTP(w, r)
	})
}

// The error helper sends a specific status code and corresponding messages
// to the user. This should be used to send responses when there's a problem with the
// request that the user sent. If no messages are provided, a default one will be generated
// from the http code.
func (app *application) error(w http.ResponseWriter, r *http.Request, status int, errors ...string) {
	if len(errors) == 0 {
		errors = append(errors, http.StatusText(status))
	}

	// if api endpoint, return json instead
	if strings.HasPrefix(r.URL.EscapedPath(), fmt.Sprintf("%s/", api)) {
		env := envelope{"errors": errors}

		err := app.writeJSON(w, status, env, nil)
		if err != nil {
			app.logger.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
		}

		return
	}

	// render error page
	w.WriteHeader(status)
	app.render(w, r, "error", &data{
		Errors: errors,
	})
}

func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	app.logger.Output(2, trace)

	msg := "The server encountered a problem and could not process your request."
	app.error(w, r, http.StatusInternalServerError, msg)
}

// notFound will be used to send a 404 Not Found status code and response to the client.
func (app *application) notFound(w http.ResponseWriter, r *http.Request) {
	msg := "The requested resource could not be found."
	app.error(w, r, http.StatusNotFound, msg)
}

// methodNotAllowed is used to send a 405 Method Not Allowed status code and response to the client.
func (app *application) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	msg := fmt.Sprintf("The %s method is not supported for this resource.", r.Method)
	app.error(w, r, http.StatusMethodNotAllowed, msg)
}

// badRequest is used to send a 400 Bad Request status code and response to the client.
func (app *application) badRequest(w http.ResponseWriter, r *http.Request, err error) {
	app.error(w, r, http.StatusBadRequest, err.Error())
}

// render will render the response.
// Write the template to the buffer, instead of straight to the http.ResponseWriter.
// This allows to deal with runtime errors in the rendering of the template.
func (app *application) render(w http.ResponseWriter, r *http.Request, name string, data *data) {
	buf := new(bytes.Buffer)

	tmpl := app.templates[name]
	if tmpl == nil {
		app.serverError(w, r, errors.New("template not found"))
		return
	}

	err := tmpl.Execute(buf, data)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	buf.WriteTo(w)
}

func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	// whitespace is added to the encoded json
	js, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// append a new line to make it nicer in terminal apps
	js = append(js, '\n')

	for k, v := range headers {
		w.Header()[k] = v
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)

	return nil
}

// readJSON decodes json from the body to a destination and checks for errors.
func (app *application) readJSON(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	maxBytes := 1_048_576 // 1MB
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("Body contains badly-formed JSON (at character %d).", syntaxError.Offset)

		// For more info see https://github.com/golang/go/issues/25956.
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("Body contains badly-formed JSON.")

		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("Body contains incorrect JSON type for field %q.", unmarshalTypeError.Field)
			}
			return fmt.Errorf("Body contains incorrect JSON type (at character %d).", unmarshalTypeError.Offset)

		case errors.Is(err, io.EOF):
			return errors.New("Body must not be empty.")

		// workaround for https://github.com/golang/go/issues/29035
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("Body contains unknown key %s.", fieldName)

		case err.Error() == "http: request body too large":
			return fmt.Errorf("Body must not be larger than %d bytes.", maxBytes)

		case errors.As(err, &invalidUnmarshalError):
			panic(err)

		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("Body must only contain a single JSON value.")
	}

	return nil
}

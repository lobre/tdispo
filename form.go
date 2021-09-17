package main

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Form will validate form data against a particular set of rules.
// If an error occurs, it will store an error message associated with
// the field.
type Form struct {
	url.Values
	errors map[string][]string
}

// New creates a new Form taking data as entry.
func NewForm(data url.Values) *Form {
	return &Form{
		data,
		map[string][]string{},
	}
}

// Error retrieves the first error message for a given
// field from the errors map.
func (f *Form) Error(field string) string {
	errors := f.errors[field]
	if len(errors) == 0 {
		return ""
	}
	return errors[0]
}

// Required checks that specific fields in the form
// data are present and not blank. If any fields fail this check,
// add the appropriate message to the form errors.
func (f *Form) Required(fields ...string) {
	for _, field := range fields {
		value := f.Get(field)
		if strings.TrimSpace(value) == "" {
			f.CustomError(field, "This field cannot be blank")
		}
	}
}

// MinLength checks that a specific field in the form contains
// a minimum number of characters. If the check fails, then add
// the appropriate message to the form errors.
func (f *Form) MinLength(field string, d int) {
	value := f.Get(field)
	if value == "" {
		return
	}
	if utf8.RuneCountInString(value) < d {
		f.CustomError(field, fmt.Sprintf("This field is too short (minimum is %d characters)", d))
	}
}

// MaxLength checks that a specific field in the form contains
// a maximum number of characters. If the check fails, then add
// the appropriate message to the form errors.
func (f *Form) MaxLength(field string, d int) {
	value := f.Get(field)
	if value == "" {
		return
	}
	// check proper characters instead of bytes
	if utf8.RuneCountInString(value) > d {
		f.CustomError(field, fmt.Sprintf("This field is too long (maximum is %d characters)", d))
	}
}

// PermittedValues checks that a specific field in the form matches
// one of a set of specific permitted values. If the check fails,
// then add the appropriate message to the form errors.
func (f *Form) PermittedValues(field string, opts ...string) {
	value := f.Get(field)
	if value == "" {
		return
	}
	for _, opt := range opts {
		if value == opt {
			return
		}
	}
	f.CustomError(field, "This field is invalid")
}

// MatchesPattern checks that a specific field in the form matches
// a regular expression. If the check fails, then add the appropriate
// message to the form errors.
func (f *Form) MatchesPattern(field string, pattern *regexp.Regexp) {
	value := f.Get(field)
	if value == "" {
		return
	}
	if !pattern.MatchString(value) {
		f.CustomError(field, "This field is invalid")
	}
}

// IsEmail checks that a specific field in the form is a correct email.
func (f *Form) IsEmail(field string) {
	_, err := mail.ParseAddress(field)
	if err != nil {
		f.CustomError(field, "This field is not a valid email")
	}
}

// CustomError adds a specific error for a field.
func (f *Form) CustomError(field, msg string) {
	f.errors[field] = append(f.errors[field], msg)
}

// Valid returns true if there are no errors in the form.
func (f *Form) Valid() bool {
	return len(f.errors) == 0
}

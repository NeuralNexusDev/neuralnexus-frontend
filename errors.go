package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
	mw "github.com/p0t4t0sandwich/neuralnexus-frontend/middleware"
)

type pageHandler func(http.ResponseWriter, *http.Request, apiSession) error

var pageRequestTimeout = 30 * time.Second

// ServeHTTP gives the handler the request time limit and the caller's session, and writes the error it returns.
func (h pageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), pageRequestTimeout)
	defer cancel()
	r = r.WithContext(ctx)
	if err := h(w, r, apiSession{r: r}); err != nil {
		writeError(w, r, err)
	}
}

type pageError struct {
	cause    error
	prefix   string
	statusID string
	parts    []templ.Component
	field    func(message string) templ.Component
	later    func() []templ.Component
}

func pageFail(err error) *pageError {
	var failure *pageError
	if errors.As(err, &failure) {
		return failure
	}
	return &pageError{cause: err}
}

func (e *pageError) Error() string {
	_, message := e.statusMessage()
	return message
}

func (e *pageError) Unwrap() error {
	return e.cause
}

func (e *pageError) clearStatus(id string) *pageError {
	e.statusID = id
	return e
}

func (e *pageError) restore(parts ...templ.Component) *pageError {
	e.parts = append(e.parts, parts...)
	return e
}

func (e *pageError) restoreLater(build func() []templ.Component) *pageError {
	e.later = build
	return e
}

func (e *pageError) flagField(field func(message string) templ.Component) *pageError {
	e.field = field
	return e
}

func (e *pageError) afterWrite() *pageError {
	e.prefix = "The change was made, but the page could not be refreshed: "
	return e
}

func (e *pageError) unauthorized() bool {
	return errors.Is(e.cause, errUnauthorized)
}

func (e *pageError) statusMessage() (int, string) {
	var failure *apiError
	if errors.As(e.cause, &failure) {
		return failure.Status, e.prefix + failure.Message
	}
	return http.StatusInternalServerError, e.prefix + "Something went wrong"
}

func (e *pageError) refusal() string {
	var failure *apiError
	if errors.As(e.cause, &failure) {
		switch failure.Status {
		case http.StatusBadRequest, http.StatusConflict, http.StatusUnprocessableEntity:
			return failure.Message
		}
	}
	return ""
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	failure := pageFail(err)
	if failure.unauthorized() {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	status, message := failure.statusMessage()
	if status >= http.StatusInternalServerError {
		logFailure(r, status, failure)
	}
	parts := []templ.Component{components.ErrorText(message)}
	if failure.statusID != "" {
		parts = append(parts, components.StatusLine(failure.statusID, ""))
	}
	parts = append(parts, failure.parts...)
	if failure.field != nil {
		if refused := failure.refusal(); refused != "" {
			parts = append(parts, failure.field(refused))
		}
	}
	if failure.later != nil {
		parts = append(parts, failure.later()...)
	}
	w.Header().Set("HX-Retarget", "#page-error")
	w.Header().Set("HX-Reswap", "innerHTML")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	render(w, r, parts...)
}

func logFailure(r *http.Request, status int, err error) {
	if errors.Is(r.Context().Err(), context.Canceled) {
		return
	}
	var failure *apiError
	call := ""
	if errors.As(err, &failure) {
		call = failure.Method + " " + failure.Path
	}
	cause := err
	for errors.Unwrap(cause) != nil {
		cause = errors.Unwrap(cause)
	}
	log.Printf("request failed: request_id=%v status=%d api=%q cause=%v", r.Context().Value(mw.RequestIDKey), status, call, cause)
}

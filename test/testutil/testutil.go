// Package testutil holds the test helpers that need nothing but the standard library.
package testutil

import (
	"net/http"
	"net/http/httptest"
)

// NewRequest returns a GET / request that carries the given cookies.
func NewRequest(cookies ...*http.Cookie) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	return req
}

// SessionCookie returns the cookie named session with the given value.
func SessionCookie(value string) *http.Cookie {
	return &http.Cookie{Name: "session", Value: value}
}

// HangHandler signals on started, which can be nil, and then blocks until the request context ends or release closes.
func HangHandler(started chan<- struct{}, release <-chan struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if started != nil {
			select {
			case started <- struct{}{}:
			case <-r.Context().Done():
				return
			}
		}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}
}

// Package testutil holds the test helpers that need nothing but the standard library.
package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

// TagByID returns the opening tag of the element whose id attribute is id, or an empty string when there is none.
func TagByID(html, id string) string {
	at := strings.Index(html, ` id="`+id+`"`)
	if at < 0 {
		return ""
	}
	start := strings.LastIndex(html[:at], "<")
	if start < 0 {
		return ""
	}
	var quote byte
	for i := at; i < len(html); i++ {
		switch c := html[i]; {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '>':
			return html[start : i+1]
		}
	}
	return ""
}

// JSONString returns v encoded as JSON and fails the test when it cannot be.
func JSONString(t testing.TB, v any) string {
	t.Helper()
	encoded, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal(%#v): %v", v, err)
	}
	return string(encoded)
}

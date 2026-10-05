// Package fakeapi is a fake nn-api for tests.
package fakeapi

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/config"
)

// Call is a request that the fake API received.
type Call struct {
	Method string
	// URI is the request URI below /api/v1, with its query string.
	URI    string
	Body   string
	Header http.Header
}

// FakeAPI answers the nn-api routes that a test sets up and records every request. A path outside /api/v1 answers 404.
type FakeAPI struct {
	// URL is the base URL of the fake server.
	URL string

	mu       sync.Mutex
	calls    []Call
	routes   map[string]http.HandlerFunc
	fallback http.HandlerFunc
}

// NewFakeAPI starts a fake API, points config.APIURL at it, and closes it and restores config.APIURL when the test ends.
func NewFakeAPI(t testing.TB) *FakeAPI {
	t.Helper()
	f := &FakeAPI{routes: map[string]http.HandlerFunc{}}
	f.fallback = answer(http.StatusInternalServerError, `{"detail":"unexpected call"}`)
	server := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(server.Close)
	f.URL = server.URL
	PointAPIAt(t, server.URL)
	return f
}

// PointAPIAt sets config.APIURL to rawURL and restores it when the test ends.
func PointAPIAt(t testing.TB, rawURL string) {
	t.Helper()
	previous := config.APIURL
	config.APIURL = rawURL
	t.Cleanup(func() { config.APIURL = previous })
}

// On answers a route, written as "METHOD /path" without /api/v1, with a status and a JSON body.
func (f *FakeAPI) On(route string, status int, body string) {
	f.Handle(route, answer(status, body))
}

// Problem answers a route with a status and a problem+json body that holds detail.
func (f *FakeAPI) Problem(route string, status int, detail string) {
	f.On(route, status, fmt.Sprintf(`{"detail":%q}`, detail))
}

// Handle answers a route with a custom handler. The request is recorded before the handler runs.
func (f *FakeAPI) Handle(route string, h http.HandlerFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes[route] = h
}

// Refuse answers a route with a status and a problem+json body that holds detail, or an empty body when detail is empty.
func (f *FakeAPI) Refuse(route string, status int, detail string) {
	if detail == "" {
		f.On(route, status, ``)
		return
	}
	f.Problem(route, status, detail)
}

// AssertLines fails the test unless the recorded calls are exactly want, in order, and none when want is empty.
func (f *FakeAPI) AssertLines(t testing.TB, want ...string) {
	t.Helper()
	if got := f.Lines(); !slices.Equal(got, want) {
		t.Errorf("API calls = %q, want %q", got, want)
	}
}

// AssertContentType fails the test unless the call's Content-Type header is want, or absent when want is empty.
func AssertContentType(t testing.TB, call Call, want string) {
	t.Helper()
	got := call.Header.Values("Content-Type")
	switch {
	case want == "" && len(got) != 0:
		t.Errorf("Content-Type = %q, want none", got)
	case want != "" && call.Header.Get("Content-Type") != want:
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
}

// Calls returns a copy of the requests that the fake API received, oldest first.
func (f *FakeAPI) Calls() []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// Lines returns one string per recorded call, oldest first, as "METHOD URI", followed by a space and the body when the body is not empty.
func (f *FakeAPI) Lines() []string {
	calls := f.Calls()
	lines := make([]string, len(calls))
	for i, call := range calls {
		lines[i] = call.Method + " " + call.URI
		if call.Body != "" {
			lines[i] += " " + call.Body
		}
	}
	return lines
}

func (f *FakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	f.mu.Lock()
	f.calls = append(f.calls, Call{
		Method: r.Method,
		URI:    strings.TrimPrefix(r.URL.RequestURI(), "/api/v1"),
		Body:   string(body),
		Header: r.Header.Clone(),
	})
	handler, ok := f.routes[r.Method+" "+strings.TrimPrefix(r.URL.EscapedPath(), "/api/v1")]
	f.mu.Unlock()
	switch {
	case !strings.HasPrefix(r.URL.Path, "/api/v1/"):
		handler = answer(http.StatusNotFound, `{"detail":"not an nn-api route"}`)
	case !ok:
		handler = f.fallback
	}
	handler(w, r)
}

func answer(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if status >= http.StatusBadRequest {
			w.Header().Set("Content-Type", "application/problem+json")
		} else {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}
}

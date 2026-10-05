package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/config"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil/fakeapi"
)

const testFallback = "Failed to save the role"

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type closeCounter struct {
	io.ReadCloser
	closes *int
}

func (c *closeCounter) Close() error {
	*c.closes++
	return c.ReadCloser.Close()
}

func newSession(r *http.Request) apiSession {
	return apiSession{r: r}
}

func assertAPIError(t *testing.T, err error, status int, message, method, path string) *apiError {
	t.Helper()
	var got *apiError
	if !errors.As(err, &got) {
		t.Fatalf("error = %#v, want an *apiError", err)
	}
	if got.Status != status || got.Message != message || got.Method != method || got.Path != path {
		t.Fatalf("apiError = {Status: %d, Message: %q, Method: %q, Path: %q}, want {Status: %d, Message: %q, Method: %q, Path: %q}",
			got.Status, got.Message, got.Method, got.Path, status, message, method, path)
	}
	return got
}

func swapAPIClient(t *testing.T, mutate func(*http.Client)) {
	t.Helper()
	previous := apiClient
	swapped := *previous
	mutate(&swapped)
	apiClient = &swapped
	t.Cleanup(func() { apiClient = previous })
}

func TestAPIError(t *testing.T) {
	t.Run("AP-01_Error_returns_only_the_message", func(t *testing.T) {
		e := &apiError{Status: 502, Message: "Failed to load roles", Method: "GET", Path: "/roles", Err: errors.New("dial tcp 10.0.0.1:443 refused")}
		if got := e.Error(); got != "Failed to load roles" {
			t.Errorf("Error() = %q", got)
		}
	})

	t.Run("AP-02_Unwrap_returns_the_stored_cause", func(t *testing.T) {
		cause := errors.New("cause")
		e := &apiError{Err: cause}
		if errors.Unwrap(e) != cause {
			t.Errorf("Unwrap() = %v, want the cause", errors.Unwrap(e))
		}
		if !errors.Is(e, cause) {
			t.Error("errors.Is(e, cause) = false")
		}
		if errors.Is(e, errUnauthorized) {
			t.Error("errors.Is(e, errUnauthorized) = true")
		}
	})

	t.Run("AP-03_Unwrap_without_a_cause_returns_nil", func(t *testing.T) {
		e := &apiError{Status: 404, Message: "x"}
		if got := errors.Unwrap(e); got != nil {
			t.Errorf("Unwrap() = %v, want nil", got)
		}
	})
}

func TestInvalidInput(t *testing.T) {
	t.Run("AP-04_builds_a_400_apiError_with_the_message", func(t *testing.T) {
		err := invalidInput("The form could not be read")
		got := assertAPIError(t, err, 400, "The form could not be read", "", "")
		if got.Err != nil {
			t.Errorf("Err = %v, want nil", got.Err)
		}
		if err.Error() != "The form could not be read" {
			t.Errorf("Error() = %q", err.Error())
		}
	})
}

func TestAPISessionCall(t *testing.T) {
	t.Run("AP-05_GET_returns_the_response_body_unchanged", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles", 200, `{"a":1}`)
		data, err := newSession(testutil.NewRequest()).call("GET", "/roles", nil, testFallback)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if string(data) != `{"a":1}` {
			t.Errorf("data = %q", data)
		}
		calls := f.Calls()
		if len(calls) != 1 || calls[0].Method != "GET" || calls[0].URI != "/roles" {
			t.Errorf("calls = %+v, want one GET /api/v1/roles", calls)
		}
	})

	t.Run("AP-06_a_query_string_in_the_path_reaches_the_API", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /users", 200, `[]`)
		if _, err := newSession(testutil.NewRequest()).call("GET", "/users?limit=50&offset=100", nil, testFallback); err != nil {
			t.Fatalf("err = %v", err)
		}
		calls := f.Calls()
		if len(calls) != 1 || calls[0].URI != "/users?limit=50&offset=100" {
			t.Errorf("calls = %+v, want the query string intact", calls)
		}
	})

	t.Run("AP-07_a_body_is_sent_as_JSON_with_the_matching_headers", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("POST /roles", 201, `{}`)
		if _, err := newSession(testutil.NewRequest()).call("POST", "/roles", map[string]string{"name": "mod"}, testFallback); err != nil {
			t.Fatalf("err = %v", err)
		}
		calls := f.Calls()
		if len(calls) != 1 {
			t.Fatalf("calls = %+v, want one", calls)
		}
		call := calls[0]
		if call.Method != "POST" || call.Body != `{"name":"mod"}` {
			t.Errorf("call = %s %q, want POST with the JSON body", call.Method, call.Body)
		}
		if got := call.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		if got := call.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q", got)
		}
	})

	t.Run("AP-08_a_nil_body_sends_no_body_and_no_Content-Type", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("DELETE /roles/1", 204, ``)
		if _, err := newSession(testutil.NewRequest()).call("DELETE", "/roles/1", nil, testFallback); err != nil {
			t.Fatalf("err = %v", err)
		}
		calls := f.Calls()
		if len(calls) != 1 {
			t.Fatalf("calls = %+v, want one", calls)
		}
		if calls[0].Body != "" {
			t.Errorf("body = %q, want empty", calls[0].Body)
		}
		if got := calls[0].Header.Values("Content-Type"); len(got) != 0 {
			t.Errorf("Content-Type = %q, want none", got)
		}
		if got := calls[0].Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q", got)
		}
	})

	t.Run("AP-09_the_incoming_session_cookie_is_forwarded", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles", 200, `[]`)
		if _, err := newSession(testutil.NewRequest(testutil.SessionCookie("abc123"))).call("GET", "/roles", nil, testFallback); err != nil {
			t.Fatalf("err = %v", err)
		}
		calls := f.Calls()
		if len(calls) != 1 {
			t.Fatalf("calls = %+v, want one", calls)
		}
		seen := (&http.Request{Header: calls[0].Header}).Cookies()
		if len(seen) != 1 || seen[0].Name != "session" || seen[0].Value != "abc123" {
			t.Errorf("cookies = %v, want only session=abc123", seen)
		}
		if got := calls[0].Header.Get("Cookie"); got != "session=abc123" {
			t.Errorf("Cookie = %q", got)
		}
	})

	t.Run("AP-10_other_cookies_and_credentials_do_not_reach_the_API", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles", 200, `[]`)
		req := testutil.NewRequest(testutil.SessionCookie("abc123"), &http.Cookie{Name: "theme", Value: "dark"}, &http.Cookie{Name: "csrf", Value: "x"})
		req.Header.Set("Authorization", "Bearer t")
		if _, err := newSession(req).call("GET", "/roles", nil, testFallback); err != nil {
			t.Fatalf("err = %v", err)
		}
		calls := f.Calls()
		if len(calls) != 1 {
			t.Fatalf("calls = %+v, want one", calls)
		}
		if got := calls[0].Header.Get("Cookie"); got != "session=abc123" {
			t.Errorf("Cookie = %q, want only session=abc123", got)
		}
		if got := calls[0].Header.Values("Authorization"); len(got) != 0 {
			t.Errorf("Authorization = %q, want none", got)
		}
	})

	t.Run("AP-11_no_session_cookie_means_no_Cookie_header", func(t *testing.T) {
		cases := []struct {
			name    string
			cookies []*http.Cookie
		}{
			{"no_cookies", nil},
			{"only_theme", []*http.Cookie{{Name: "theme", Value: "dark"}}},
		}
		for _, tc := range cases {
			t.Run("AP-11_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /roles", 200, `[]`)
				data, err := newSession(testutil.NewRequest(tc.cookies...)).call("GET", "/roles", nil, testFallback)
				if err != nil || string(data) != `[]` {
					t.Fatalf("data = %q, err = %v", data, err)
				}
				calls := f.Calls()
				if len(calls) != 1 {
					t.Fatalf("calls = %+v, want one", calls)
				}
				if got := calls[0].Header.Values("Cookie"); len(got) != 0 {
					t.Errorf("Cookie = %q, want none", got)
				}
			})
		}
	})

	t.Run("AP-12_the_session_cookie_does_not_follow_a_redirect_to_another_host", func(t *testing.T) {
		seen := make(chan http.Header, 1)
		other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen <- r.Header.Clone()
			fmt.Fprint(w, `{}`)
		}))
		t.Cleanup(other.Close)
		target := strings.Replace(other.URL, "127.0.0.1", "localhost", 1) + "/x"
		f := fakeapi.NewFakeAPI(t)
		f.Handle("GET /roles", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target, http.StatusFound)
		})
		if _, err := newSession(testutil.NewRequest(testutil.SessionCookie("abc123"))).call("GET", "/roles", nil, testFallback); err != nil {
			t.Fatalf("err = %v", err)
		}
		select {
		case header := <-seen:
			if got := header.Values("Cookie"); len(got) != 0 {
				t.Errorf("the other host saw Cookie %q", got)
			}
		default:
			t.Fatal("the other host was not reached")
		}
	})

	t.Run("AP-13_the_shared_client_has_a_15_second_timeout_and_no_cookie_jar", func(t *testing.T) {
		if apiClient.Timeout != 15*time.Second {
			t.Errorf("Timeout = %v, want 15s", apiClient.Timeout)
		}
		if apiClient.Jar != nil {
			t.Error("Jar is set, want nil")
		}
	})

	t.Run("AP-14_a_server_that_never_answers_ends_with_a_timeout_error", func(t *testing.T) {
		swapAPIClient(t, func(c *http.Client) { c.Timeout = 100 * time.Millisecond })
		f := fakeapi.NewFakeAPI(t)
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		f.Handle("GET /roles", testutil.HangHandler(nil, release))
		_, err := newSession(testutil.NewRequest()).call("GET", "/roles", nil, testFallback)
		got := assertAPIError(t, err, 502, testFallback, "GET", "/roles")
		var netErr net.Error
		if !errors.As(got.Err, &netErr) || !netErr.Timeout() {
			t.Errorf("Err = %v, want a net.Error timeout", got.Err)
		}
	})

	t.Run("AP-15_a_body_that_cannot_be_encoded_fails_before_any_request", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		_, err := newSession(testutil.NewRequest()).call("POST", "/roles", make(chan int), testFallback)
		got := assertAPIError(t, err, 500, testFallback, "POST", "/roles")
		var unsupported *json.UnsupportedTypeError
		if !errors.As(got.Err, &unsupported) {
			t.Errorf("Err = %v, want a *json.UnsupportedTypeError", got.Err)
		}
		if calls := f.Calls(); len(calls) != 0 {
			t.Errorf("the API saw %d requests, want none", len(calls))
		}
	})

	t.Run("AP-16_a_request_that_cannot_be_built_fails_before_any_request", func(t *testing.T) {
		cases := []struct{ name, method, path string }{
			{"bad_method", "BAD METHOD", "/x"},
			{"bad_path", "GET", "/x\ny"},
		}
		for _, tc := range cases {
			t.Run("AP-16_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				_, err := newSession(testutil.NewRequest()).call(tc.method, tc.path, nil, testFallback)
				got := assertAPIError(t, err, 502, testFallback, tc.method, tc.path)
				if got.Err == nil {
					t.Error("Err is nil, want the cause")
				}
				if calls := f.Calls(); len(calls) != 0 {
					t.Errorf("the API saw %d requests, want none", len(calls))
				}
			})
		}
	})

	t.Run("AP-17_canceling_the_incoming_request_cancels_the_API_call", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		started := make(chan struct{})
		f.Handle("GET /roles", testutil.HangHandler(started, nil))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			_, err := newSession(testutil.NewRequest().WithContext(ctx)).call("GET", "/roles", nil, testFallback)
			done <- err
		}()
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("the API did not receive the request")
		}
		cancel()
		select {
		case err := <-done:
			assertAPIError(t, err, 502, testFallback, "GET", "/roles")
			if !errors.Is(err, context.Canceled) {
				t.Errorf("err = %v, want context.Canceled", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the call did not return after the cancel")
		}
	})

	t.Run("AP-18_an_unreachable_API_maps_to_502_with_the_fallback", func(t *testing.T) {
		fakeapi.PointAPIAtClosed(t)
		_, err := newSession(testutil.NewRequest()).call("GET", "/roles", nil, testFallback)
		got := assertAPIError(t, err, 502, testFallback, "GET", "/roles")
		if got.Err == nil {
			t.Error("Err is nil, want the cause")
		}
		for _, leaked := range []string{strings.TrimPrefix(config.APIURL, "http://"), "refused"} {
			if strings.Contains(err.Error(), leaked) {
				t.Errorf("Error() = %q, holds %q", err.Error(), leaked)
			}
		}
	})

	t.Run("AP-19_a_response_body_that_breaks_mid_read_maps_to_502", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.Handle("GET /roles", func(w http.ResponseWriter, r *http.Request) {
			conn, buf, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("Hijack: %v", err)
				return
			}
			defer conn.Close()
			buf.WriteString("HTTP/1.1 200 OK\r\nContent-Length: 100\r\n\r\n0123456789")
			buf.Flush()
		})
		_, err := newSession(testutil.NewRequest()).call("GET", "/roles", nil, testFallback)
		got := assertAPIError(t, err, 502, testFallback, "GET", "/roles")
		if !errors.Is(got.Err, io.ErrUnexpectedEOF) {
			t.Errorf("Err = %v, want io.ErrUnexpectedEOF", got.Err)
		}
	})

	t.Run("AP-20_a_401_maps_to_errUnauthorized", func(t *testing.T) {
		cases := []struct{ name, body string }{
			{"empty_body", ""},
			{"detail", `{"detail":"Token expired"}`},
			{"html", "<html>Unauthorized</html>"},
		}
		for _, tc := range cases {
			t.Run("AP-20_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /roles", 401, tc.body)
				data, err := newSession(testutil.NewRequest()).call("GET", "/roles", nil, testFallback)
				if err != errUnauthorized {
					t.Errorf("err = %v, want errUnauthorized", err)
				}
				var failure *apiError
				if errors.As(err, &failure) {
					t.Errorf("err is an *apiError: %+v", failure)
				}
				if data != nil {
					t.Errorf("data = %q, want nil", data)
				}
			})
		}
	})

	t.Run("AP-21_a_4xx_with_a_detail_carries_the_detail", func(t *testing.T) {
		for _, status := range []int{400, 403, 404, 409, 422} {
			t.Run(fmt.Sprintf("AP-21_%d", status), func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Problem("PUT /roles/1", status, "Role name taken")
				_, err := newSession(testutil.NewRequest()).call("PUT", "/roles/1", map[string]string{"name": "mod"}, testFallback)
				got := assertAPIError(t, err, status, "Role name taken", "PUT", "/roles/1")
				if got.Err != nil || errors.Unwrap(err) != nil {
					t.Errorf("Err = %v, want nil", got.Err)
				}
			})
		}
	})

	t.Run("AP-22_a_5xx_with_a_detail_carries_the_detail", func(t *testing.T) {
		for _, status := range []int{500, 502, 503} {
			t.Run(fmt.Sprintf("AP-22_%d", status), func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Problem("GET /roles", status, "database down")
				_, err := newSession(testutil.NewRequest()).call("GET", "/roles", nil, testFallback)
				got := assertAPIError(t, err, status, "database down", "GET", "/roles")
				if got.Err != nil {
					t.Errorf("Err = %v, want nil", got.Err)
				}
			})
		}
	})

	t.Run("AP-23_an_error_status_without_a_usable_detail_falls_back", func(t *testing.T) {
		bodies := []struct{ name, body string }{
			{"empty_body", ""},
			{"plain_text", "gateway trouble"},
			{"html", "<html>Bad Gateway</html>"},
			{"no_detail", `{"error":"x"}`},
			{"empty_detail", `{"detail":""}`},
			{"null_detail", `{"detail":null}`},
			{"list_detail", `{"detail":[{"msg":"bad"}]}`},
			{"number_detail", `{"detail":42}`},
		}
		for _, status := range []int{404, 503} {
			for _, tc := range bodies {
				t.Run(fmt.Sprintf("AP-23_%d_%s", status, tc.name), func(t *testing.T) {
					f := fakeapi.NewFakeAPI(t)
					f.On("GET /roles", status, tc.body)
					_, err := newSession(testutil.NewRequest()).call("GET", "/roles", nil, testFallback)
					assertAPIError(t, err, status, testFallback, "GET", "/roles")
				})
			}
		}
	})

	t.Run("AP-24_the_fallback_is_ignored_for_a_401", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles", 401, ``)
		_, err := newSession(testutil.NewRequest()).call("GET", "/roles", nil, "Failed to load roles")
		if err == nil || err.Error() != "not signed in" {
			t.Fatalf("err = %v, want not signed in", err)
		}
		if strings.Contains(err.Error(), "Failed to load roles") {
			t.Errorf("Error() = %q, holds the fallback", err.Error())
		}
	})

	t.Run("AP-25_an_empty_fallback_with_no_detail_gives_an_empty_message", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles", 500, ``)
		_, err := newSession(testutil.NewRequest()).call("GET", "/roles", nil, "")
		assertAPIError(t, err, 500, "", "GET", "/roles")
		if err.Error() != "" {
			t.Errorf("Error() = %q, want empty", err.Error())
		}
	})

	t.Run("AP-26_success_statuses_return_the_body_bytes", func(t *testing.T) {
		cases := []struct {
			name   string
			status int
			body   string
		}{
			{"created_with_a_body", 201, `{"id":1}`},
			{"no_content", 204, ``},
		}
		for _, tc := range cases {
			t.Run("AP-26_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("POST /roles", tc.status, tc.body)
				data, err := newSession(testutil.NewRequest()).call("POST", "/roles", map[string]string{"name": "mod"}, testFallback)
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				if string(data) != tc.body {
					t.Errorf("data = %q, want %q", data, tc.body)
				}
			})
		}
	})

	t.Run("AP-27_a_response_over_1_MiB_is_cut_at_1_MiB", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles", 200, strings.Repeat("x", 1048576+100))
		data, err := newSession(testutil.NewRequest()).call("GET", "/roles", nil, testFallback)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if len(data) != 1048576 {
			t.Errorf("len(data) = %d, want 1048576", len(data))
		}
	})

	t.Run("AP-28_the_response_body_is_closed_on_success_and_on_an_error_status", func(t *testing.T) {
		for _, status := range []int{200, 500} {
			t.Run(fmt.Sprintf("AP-28_%d", status), func(t *testing.T) {
				closes := 0
				swapAPIClient(t, func(c *http.Client) {
					c.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
						res, err := http.DefaultTransport.RoundTrip(req)
						if err != nil {
							return nil, err
						}
						res.Body = &closeCounter{ReadCloser: res.Body, closes: &closes}
						return res, nil
					})
				})
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /roles", status, `{}`)
				newSession(testutil.NewRequest()).call("GET", "/roles", nil, testFallback)
				if closes != 1 {
					t.Errorf("Close was called %d times, want 1", closes)
				}
			})
		}
	})

	t.Run("AP-29_concurrent_calls_never_mix_up_session_cookies", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.Handle("GET /me", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, r.Header.Get("Cookie"))
		})
		const callers, rounds = 20, 10
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := range callers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				cookie := fmt.Sprintf("s-%d", i)
				<-start
				for range rounds {
					data, err := newSession(testutil.NewRequest(testutil.SessionCookie(cookie))).call("GET", "/me", nil, testFallback)
					if err != nil || string(data) != "session="+cookie {
						t.Errorf("caller %d got %q, err = %v", i, data, err)
						return
					}
				}
			}()
		}
		close(start)
		wg.Wait()
	})
}

func TestAPIGet(t *testing.T) {
	type role struct {
		Name string `json:"name"`
	}

	t.Run("AP-30_decodes_a_JSON_response_into_T", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles/1", 200, `{"name":"mod"}`)
		f.On("GET /roles", 200, `["a","b"]`)
		session := newSession(testutil.NewRequest())
		one, err := apiGet[role](session, "/roles/1", testFallback)
		if err != nil || one.Name != "mod" {
			t.Errorf("role = %+v, err = %v", one, err)
		}
		many, err := apiGet[[]string](session, "/roles", testFallback)
		if err != nil || len(many) != 2 || many[0] != "a" || many[1] != "b" {
			t.Errorf("roles = %v, err = %v", many, err)
		}
		calls := f.Calls()
		if len(calls) != 2 {
			t.Fatalf("calls = %+v, want two", calls)
		}
		for _, call := range calls {
			if call.Method != "GET" || call.Body != "" || len(call.Header.Values("Content-Type")) != 0 {
				t.Errorf("call = %+v, want a GET without a body or Content-Type", call)
			}
		}
	})

	t.Run("AP-31_a_401_passes_through_as_errUnauthorized", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles", 401, ``)
		got, err := apiGet[[]string](newSession(testutil.NewRequest()), "/roles", testFallback)
		if err != errUnauthorized {
			t.Errorf("err = %v, want errUnauthorized", err)
		}
		if got != nil {
			t.Errorf("result = %v, want nil", got)
		}
	})

	t.Run("AP-32_an_error_status_passes_through_unchanged", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.Problem("GET /roles/9", 404, "Role not found")
		got, err := apiGet[role](newSession(testutil.NewRequest()), "/roles/9", testFallback)
		assertAPIError(t, err, 404, "Role not found", "GET", "/roles/9")
		if got != (role{}) {
			t.Errorf("result = %+v, want the zero value", got)
		}
	})

	t.Run("AP-33_a_200_body_that_does_not_decode_maps_to_502", func(t *testing.T) {
		cases := []struct {
			name, body string
			wantSyntax bool
		}{
			{"not_json", "not json", true},
			{"wrong_type", `{"a":1}`, false},
		}
		for _, tc := range cases {
			t.Run("AP-33_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /roles", 200, tc.body)
				_, err := apiGet[[]string](newSession(testutil.NewRequest()), "/roles", testFallback)
				got := assertAPIError(t, err, 502, testFallback, "GET", "/roles")
				var syntax *json.SyntaxError
				var mismatch *json.UnmarshalTypeError
				if tc.wantSyntax && !errors.As(got.Err, &syntax) {
					t.Errorf("Err = %v, want a *json.SyntaxError", got.Err)
				}
				if !tc.wantSyntax && !errors.As(got.Err, &mismatch) {
					t.Errorf("Err = %v, want a *json.UnmarshalTypeError", got.Err)
				}
			})
		}
	})

	t.Run("AP-34_an_empty_200_body_is_a_decode_failure", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles/1", 200, ``)
		_, err := apiGet[role](newSession(testutil.NewRequest()), "/roles/1", testFallback)
		got := assertAPIError(t, err, 502, testFallback, "GET", "/roles/1")
		if got.Err == nil {
			t.Error("Err is nil, want the decode error")
		}
	})

	t.Run("AP-35_a_JSON_body_over_1_MiB_fails_to_decode", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /roles", 200, `["`+strings.Repeat("x", 1048576)+`"]`)
		_, err := apiGet[[]string](newSession(testutil.NewRequest()), "/roles", testFallback)
		got := assertAPIError(t, err, 502, testFallback, "GET", "/roles")
		if got.Err == nil {
			t.Error("Err is nil, want the decode error")
		}
	})
}

func TestAPISend(t *testing.T) {
	type created struct {
		ID int `json:"id"`
	}

	t.Run("AP-36_sends_the_method_and_JSON_body_and_decodes_the_response", func(t *testing.T) {
		for _, method := range []string{"PUT", "PATCH", "POST"} {
			t.Run("AP-36_"+method, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On(method+" /roles/1", 200, `{"id":7}`)
				got, err := apiSend[created](newSession(testutil.NewRequest()), method, "/roles/1", map[string]string{"name": "mod"}, testFallback)
				if err != nil || got.ID != 7 {
					t.Errorf("result = %+v, err = %v", got, err)
				}
				calls := f.Calls()
				if len(calls) != 1 {
					t.Fatalf("calls = %+v, want one", calls)
				}
				if calls[0].Method != method || calls[0].Body != `{"name":"mod"}` {
					t.Errorf("call = %s %q", calls[0].Method, calls[0].Body)
				}
				if got := calls[0].Header.Get("Content-Type"); got != "application/json" {
					t.Errorf("Content-Type = %q", got)
				}
			})
		}
	})

	t.Run("AP-37_an_empty_or_whitespace_response_gives_a_zero_value", func(t *testing.T) {
		cases := []struct {
			name   string
			status int
			body   string
		}{
			{"no_content", 204, ``},
			{"whitespace", 200, "  \n"},
		}
		for _, tc := range cases {
			t.Run("AP-37_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("DELETE /roles/1", tc.status, tc.body)
				got, err := apiSend[created](newSession(testutil.NewRequest()), "DELETE", "/roles/1", nil, testFallback)
				if err != nil || got != (created{}) {
					t.Errorf("result = %+v, err = %v", got, err)
				}
				calls := f.Calls()
				if len(calls) != 1 || len(calls[0].Header.Values("Content-Type")) != 0 {
					t.Errorf("calls = %+v, want one without a Content-Type", calls)
				}
			})
		}
	})

	t.Run("AP-38_a_2xx_body_that_does_not_decode_maps_to_502_with_the_callers_method", func(t *testing.T) {
		cases := []struct {
			name, body string
			wantSyntax bool
		}{
			{"not_json", "not json", true},
			{"wrong_type", `[1]`, false},
		}
		for _, tc := range cases {
			t.Run("AP-38_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("PATCH /roles/1", 200, tc.body)
				_, err := apiSend[created](newSession(testutil.NewRequest()), "PATCH", "/roles/1", map[string]string{"name": "mod"}, testFallback)
				got := assertAPIError(t, err, 502, testFallback, "PATCH", "/roles/1")
				var syntax *json.SyntaxError
				var mismatch *json.UnmarshalTypeError
				if tc.wantSyntax && !errors.As(got.Err, &syntax) {
					t.Errorf("Err = %v, want a *json.SyntaxError", got.Err)
				}
				if !tc.wantSyntax && !errors.As(got.Err, &mismatch) {
					t.Errorf("Err = %v, want a *json.UnmarshalTypeError", got.Err)
				}
			})
		}
	})

	t.Run("AP-39_a_401_passes_through_as_errUnauthorized", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("POST /roles", 401, ``)
		got, err := apiSend[created](newSession(testutil.NewRequest()), "POST", "/roles", map[string]string{"name": "mod"}, testFallback)
		if err != errUnauthorized {
			t.Errorf("err = %v, want errUnauthorized", err)
		}
		if got != (created{}) {
			t.Errorf("result = %+v, want the zero value", got)
		}
	})

	t.Run("AP-40_an_error_status_passes_through_unchanged", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.Problem("POST /roles", 409, "Role name taken")
		got, err := apiSend[created](newSession(testutil.NewRequest()), "POST", "/roles", map[string]string{"name": "mod"}, testFallback)
		assertAPIError(t, err, 409, "Role name taken", "POST", "/roles")
		if got != (created{}) {
			t.Errorf("result = %+v, want the zero value", got)
		}
	})

	t.Run("AP-41_a_body_that_cannot_be_encoded_fails_before_any_request", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		_, err := apiSend[created](newSession(testutil.NewRequest()), "POST", "/roles", make(chan int), testFallback)
		assertAPIError(t, err, 500, testFallback, "POST", "/roles")
		if calls := f.Calls(); len(calls) != 0 {
			t.Errorf("the API saw %d requests, want none", len(calls))
		}
	})
}

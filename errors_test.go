package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
	mw "github.com/p0t4t0sandwich/neuralnexus-frontend/middleware"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil"
)

func failureRequest(ctx context.Context) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/x", nil).WithContext(ctx)
}

func writeFailure(err error) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	writeError(rec, failureRequest(context.WithValue(context.Background(), mw.RequestIDKey, 77)), err)
	return rec
}

func TestWriteErrorSignedOutRedirectsToLogin(t *testing.T) {
	t.Run("ER-01 a 401 redirects to the login page and runs nothing else", func(t *testing.T) {
		logged := captureLog(t)
		built := false
		failure := pageFail(fmt.Errorf("load: %w", errUnauthorized)).
			clearStatus("s").
			restore(components.StatusLine("kept", "x")).
			restoreLater(func() []templ.Component {
				built = true
				return nil
			})
		rec := writeFailure(failure)
		assertLoginRedirect(t, rec)
		if rec.Header().Get("HX-Retarget") != "" || rec.Header().Get("HX-Reswap") != "" {
			t.Errorf("headers = %v, want no retarget or reswap", rec.Header())
		}
		if built {
			t.Error("restoreLater ran on a 401")
		}
		if logged.Len() != 0 {
			t.Errorf("log = %q, want empty", logged.String())
		}
	})
}

func TestWriteErrorAnswersWithTheAPIMessage(t *testing.T) {
	t.Run("ER-02 a 4xx keeps its status and message and is not logged", func(t *testing.T) {
		logged := captureLog(t)
		rec := writeFailure(&apiError{Status: 404, Message: "User not found", Method: "GET", Path: "/users/u1"})
		assertFailure(t, rec, 404, "User not found")
		if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Errorf("Content-Type = %q, want text/html; charset=utf-8", got)
		}
		if got := strings.TrimSpace(rec.Body.String()); got != "User not found" {
			t.Errorf("body = %q, want only the message", got)
		}
		if logged.Len() != 0 {
			t.Errorf("log = %q, want empty", logged.String())
		}
	})

	t.Run("ER-03 a 5xx keeps its status and message and is logged with the request, call and cause", func(t *testing.T) {
		logged := captureLog(t)
		cause := errors.New("connection refused")
		rec := writeFailure(&apiError{Status: 502, Message: "Failed to load roles", Method: "GET", Path: "/roles", Err: cause})
		assertFailure(t, rec, 502, "Failed to load roles")
		for _, want := range []string{"request failed", "request_id=77", "status=502", `api="GET /roles"`, "cause=connection refused"} {
			if !strings.Contains(logged.String(), want) {
				t.Errorf("log lacks %q: %q", want, logged.String())
			}
		}
	})

	t.Run("ER-04 an error that is not an API error answers 500 with the fallback and is logged", func(t *testing.T) {
		logged := captureLog(t)
		rec := writeFailure(errors.New("boom"))
		assertFailure(t, rec, 500, "Something went wrong")
		assertBodyLacks(t, rec, "boom")
		for _, want := range []string{"status=500", `api=""`, "cause=boom"} {
			if !strings.Contains(logged.String(), want) {
				t.Errorf("log lacks %q: %q", want, logged.String())
			}
		}
	})

	t.Run("ER-05 a 5xx on a cancelled request is not logged", func(t *testing.T) {
		logged := captureLog(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		rec := httptest.NewRecorder()
		writeError(rec, failureRequest(ctx), &apiError{Status: 502, Message: "Failed to load roles", Err: context.Canceled})
		assertStatusCode(t, rec, 502)
		if logged.Len() != 0 {
			t.Errorf("log = %q, want empty", logged.String())
		}
	})

	t.Run("ER-06 an API error wrapped by the caller still gives its status and message", func(t *testing.T) {
		rec := writeFailure(fmt.Errorf("saving: %w", &apiError{Status: 403, Message: "Forbidden"}))
		assertFailure(t, rec, 403, "Forbidden")
	})
}

func TestPageErrorRestoresWhatTheFailureNames(t *testing.T) {
	refused := &apiError{Status: 409, Message: "Name taken"}

	t.Run("ER-07 clearStatus adds an empty status line after the message", func(t *testing.T) {
		rec := writeFailure(pageFail(refused).clearStatus("role-status"))
		assertFailure(t, rec, 409, "Name taken")
		if tag := testutil.TagByID(rec.Body.String(), "role-status"); tag == "" || !strings.Contains(tag, "hx-swap-oob") {
			t.Errorf("no out-of-band status line in %q", rec.Body.String())
		}
		assertBodyHas(t, rec, "<p id=\"role-status\" hx-swap-oob=\"innerHTML\"></p>")
	})

	t.Run("ER-08 restore parts follow the message in the order given", func(t *testing.T) {
		rec := writeFailure(pageFail(refused).restore(components.StatusLine("first", ""), components.StatusLine("second", "")).restore(components.StatusLine("third", "")))
		assertFailure(t, rec, 409, "Name taken")
		body := rec.Body.String()
		last := strings.Index(body, "Name taken")
		for _, id := range []string{"first", "second", "third"} {
			at := strings.Index(body, `id="`+id+`"`)
			if at < last {
				t.Fatalf("%s is missing or out of order in %q", id, body)
			}
			last = at
		}
	})

	t.Run("ER-09 flagField adds the flagged field for 400, 409 and 422", func(t *testing.T) {
		for _, status := range []int{400, 409, 422} {
			t.Run(fmt.Sprint(status), func(t *testing.T) {
				failure := pageFail(&apiError{Status: status, Message: "Bad name"}).
					flagField(func(message string) templ.Component { return components.StatusLine("name-error", message) })
				rec := writeFailure(failure)
				assertFailure(t, rec, status, "Bad name")
				assertBodyHas(t, rec, `<p id="name-error" hx-swap-oob="innerHTML">Bad name</p>`)
			})
		}
	})

	t.Run("ER-10 flagField adds nothing for other statuses or a non-API error", func(t *testing.T) {
		for name, err := range map[string]error{
			"403":     &apiError{Status: 403, Message: "Forbidden"},
			"500":     &apiError{Status: 500, Message: "database down"},
			"non-api": errors.New("boom"),
		} {
			t.Run(name, func(t *testing.T) {
				captureLog(t)
				failure := pageFail(err).flagField(func(message string) templ.Component { return components.StatusLine("name-error", message) })
				assertBodyLacks(t, writeFailure(failure), "name-error")
			})
		}
	})

	t.Run("ER-11 restoreLater builds its parts once, after the other parts", func(t *testing.T) {
		calls := 0
		failure := pageFail(refused).
			restoreLater(func() []templ.Component {
				calls++
				return []templ.Component{components.StatusLine("late", "")}
			}).
			restore(components.StatusLine("early", ""))
		rec := writeFailure(failure)
		if calls != 1 {
			t.Errorf("restoreLater ran %d times, want 1", calls)
		}
		body := rec.Body.String()
		if early, late := strings.Index(body, `id="early"`), strings.Index(body, `id="late"`); early < 0 || late < early {
			t.Errorf("early at %d, late at %d in %q", early, late, body)
		}
	})
}

func TestPageErrorAfterWrite(t *testing.T) {
	const prefix = "The change was made, but the page could not be refreshed: "

	t.Run("ER-12 afterWrite puts the explanation before the message and keeps the status", func(t *testing.T) {
		captureLog(t)
		rec := writeFailure(pageFail(&apiError{Status: 502, Message: "Failed to load roles", Method: "GET", Path: "/roles"}).afterWrite())
		assertFailure(t, rec, 502, prefix+"Failed to load roles")
	})

	t.Run("ER-13 afterWrite leaves a 401 as a login redirect", func(t *testing.T) {
		assertLoginRedirect(t, writeFailure(pageFail(errUnauthorized).afterWrite()))
	})

	t.Run("ER-14 the log line of an afterWrite failure has the plain cause", func(t *testing.T) {
		logged := captureLog(t)
		writeFailure(pageFail(&apiError{Status: 502, Message: "Failed to load roles", Method: "GET", Path: "/roles", Err: errors.New("connection refused")}).afterWrite())
		if !strings.Contains(logged.String(), `api="GET /roles"`) || !strings.Contains(logged.String(), "cause=connection refused") || strings.Contains(logged.String(), "The change was made") {
			t.Errorf("log = %q", logged.String())
		}
	})
}

func TestPageFail(t *testing.T) {
	t.Run("ER-15 pageFail returns the pageError it is given, also when wrapped", func(t *testing.T) {
		first := pageFail(errors.New("boom")).clearStatus("s")
		if pageFail(first) != first || pageFail(fmt.Errorf("again: %w", first)) != first {
			t.Error("pageFail wrapped a pageError instead of returning it")
		}
	})

	t.Run("ER-16 pageFail keeps the cause reachable", func(t *testing.T) {
		failure := pageFail(fmt.Errorf("load: %w", &apiError{Status: 404, Message: "Not found"}))
		var got *apiError
		if !errors.As(failure, &got) || got.Status != 404 {
			t.Errorf("errors.As found %v", got)
		}
		if !pageFail(fmt.Errorf("load: %w", errUnauthorized)).unauthorized() || pageFail(errors.New("boom")).unauthorized() {
			t.Error("unauthorized is wrong")
		}
	})
}

func TestPageRouteWritesTheReturnedError(t *testing.T) {
	serve := func(h http.Handler, method string, header string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/x", strings.NewReader(url.Values{"a": {"1"}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if header != "" {
			req.Header.Set("HX-Request", header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	t.Run("ER-17 pageRoute turns a returned error into the failure response", func(t *testing.T) {
		rec := serve(pageRoute(func(http.ResponseWriter, *http.Request, apiSession) error {
			return &apiError{Status: 404, Message: "Not found"}
		}), http.MethodGet, "")
		assertFailure(t, rec, 404, "Not found")
		if got := rec.Header().Get("Cache-Control"); got != noStore {
			t.Errorf("Cache-Control = %q, want %q", got, noStore)
		}
	})

	t.Run("ER-18 pageRoute leaves the response of a handler that returns nil alone", func(t *testing.T) {
		rec := serve(pageRoute(func(w http.ResponseWriter, _ *http.Request, _ apiSession) error {
			w.WriteHeader(http.StatusTeapot)
			return nil
		}), http.MethodGet, "")
		assertStatusCode(t, rec, http.StatusTeapot)
		if rec.Header().Get("HX-Retarget") != "" || rec.Body.Len() != 0 {
			t.Errorf("headers %v, body %q", rec.Header(), rec.Body.String())
		}
	})

	t.Run("ER-19 pageAction answers 400 when the form cannot be read", func(t *testing.T) {
		called := false
		h := pageAction(func(http.ResponseWriter, *http.Request, apiSession) error {
			called = true
			return nil
		})
		req := httptest.NewRequest(http.MethodPost, "/x?a=%zz", nil)
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		assertFailure(t, rec, 400, "The form could not be read")
		if called {
			t.Error("the handler ran after the form failed to parse")
		}
	})

	t.Run("ER-20 pageAction refuses a request without HX-Request", func(t *testing.T) {
		called := false
		rec := serve(pageAction(func(http.ResponseWriter, *http.Request, apiSession) error {
			called = true
			return nil
		}), http.MethodPost, "")
		assertStatusCode(t, rec, http.StatusForbidden)
		if called {
			t.Error("the handler ran without HX-Request")
		}
	})
}

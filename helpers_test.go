package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil"
)

func serveRequest(method, target string, form url.Values) *httptest.ResponseRecorder {
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	NewWebServer("", false).Setup().ServeHTTP(rec, req)
	return rec
}

func assertStatusCode(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		body := rec.Body.String()
		if len(body) > 200 {
			body = body[:200]
		}
		t.Fatalf("status = %d, want %d, body starts %q", rec.Code, want, body)
	}
}

func assertBodyHas(t *testing.T, rec *httptest.ResponseRecorder, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("the body lacks %q:\n%s", want, rec.Body.String())
		}
	}
}

func assertBodyLacks(t *testing.T, rec *httptest.ResponseRecorder, unwanted ...string) {
	t.Helper()
	for _, bad := range unwanted {
		if strings.Contains(rec.Body.String(), bad) {
			t.Errorf("the body holds %q:\n%s", bad, rec.Body.String())
		}
	}
}

func assertFailure(t *testing.T, rec *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	assertStatusCode(t, rec, status)
	if got := rec.Header().Get("HX-Retarget"); got != "#page-error" {
		t.Errorf("HX-Retarget = %q, want #page-error", got)
	}
	if got := rec.Header().Get("HX-Reswap"); got != "innerHTML" {
		t.Errorf("HX-Reswap = %q, want innerHTML", got)
	}
	if got, _, _ := strings.Cut(rec.Body.String(), "<"); got != message {
		t.Errorf("message = %q, want %q", got, message)
	}
}

func assertLoginRedirect(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	assertStatusCode(t, rec, 401)
	if got := rec.Header().Get("HX-Redirect"); got != "/login" {
		t.Errorf("HX-Redirect = %q, want /login", got)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
}

func assertOutOfBand(t *testing.T, rec *httptest.ResponseRecorder, id string, want bool) {
	t.Helper()
	tag := testutil.TagByID(rec.Body.String(), id)
	if tag == "" {
		t.Errorf("the body has no element with id %q:\n%s", id, rec.Body.String())
		return
	}
	if got := strings.Contains(tag, " hx-swap-oob"); got != want {
		t.Errorf("hx-swap-oob on %q present = %t, want %t; tag %s", id, got, want, tag)
	}
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestBeeSuggestionsListThePendingNames(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /bee-name-generator/suggestion/100", 200, `{"suggestions":["buzz","honey"]}`)
	rec := getPage("/project/bee-name-generator/admin/suggestions")
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec, ">buzz<", ">honey<", `value="accept"`, `value="reject"`, `aria-label="Accept buzz"`, `aria-label="Reject honey"`,
		`hx-post="/project/bee-name-generator/admin/suggestions"`, `id="bee-suggestions-root" class="space-y-6 `+busyClasses+`" hx-indicator:inherited="this" hx-sync:inherited="this:drop"`)
	assertNoBody(t, rec, `id="bee-suggestions-empty"`, "autofocus", "<html")
	if got := f.uris(); len(got) != 1 || got[0] != "GET /bee-name-generator/suggestion/100" {
		t.Errorf("calls = %v", got)
	}
}

func TestBeeSuggestionsEmpty(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /bee-name-generator/suggestion/100", 200, `{"suggestions":[]}`)
	assertBody(t, getPage("/project/bee-name-generator/admin/suggestions"), `id="bee-suggestions-empty"`, "No pending suggestions")
	f.on("GET /bee-name-generator/suggestion/100", 200, `{}`)
	assertBody(t, getPage("/project/bee-name-generator/admin/suggestions"), `id="bee-suggestions-empty"`)
}

func TestBeeSuggestionsRefusedShowsTheAPIMessage(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("GET /bee-name-generator/suggestion/100", 403, "You do not have permission to review suggestions")
	rec := getPage("/project/bee-name-generator/admin/suggestions")
	assertStatus(t, rec, http.StatusForbidden)
	if got := bannerText(rec); got != "You do not have permission to review suggestions" {
		t.Errorf("body = %q", got)
	}
}

func TestBeeSuggestionsEscapeTheNames(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("GET /bee-name-generator/suggestion/100", 200, fmt.Sprintf(`{"suggestions":[%q]}`, hostile))
	rec := getPage("/project/bee-name-generator/admin/suggestions")
	assertNoBody(t, rec, "<img src=x")
	assertBody(t, rec, "&lt;img src=x")
}

func TestBeeSuggestionsSignedOutRedirectsToLogin(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("GET /bee-name-generator/suggestion/100", 401, "sign in")
	rec := getPage("/project/bee-name-generator/admin/suggestions")
	if got := rec.Header().Get("HX-Redirect"); got != "/login" {
		t.Errorf("HX-Redirect = %q", got)
	}
}

func reviewBee(form url.Values) *httptest.ResponseRecorder {
	return action(http.MethodPost, "/project/bee-name-generator/admin/suggestions", form)
}

func TestBeeReviewAcceptsAndRejectsByName(t *testing.T) {
	cases := []struct {
		action, name, want string
	}{
		{"accept", "buzz", "PUT /bee-name-generator/suggestion/buzz "},
		{"reject", "buzz", "DELETE /bee-name-generator/suggestion/buzz "},
		{"accept", "royal jelly/queen?", "PUT /bee-name-generator/suggestion/royal%20jelly%2Fqueen%3F "},
		{"reject", "100%", "DELETE /bee-name-generator/suggestion/100%25 "},
		{"accept", "..a", "PUT /bee-name-generator/suggestion/..a "},
	}
	for _, tc := range cases {
		t.Run(tc.action+" "+tc.name, func(t *testing.T) {
			f := newFakeAdmin(t)
			f.on("PUT /bee-name-generator/suggestion/"+url.PathEscape(tc.name), 200, ``)
			f.on("DELETE /bee-name-generator/suggestion/"+url.PathEscape(tc.name), 204, ``)
			f.on("GET /bee-name-generator/suggestion/100", 200, `{"suggestions":["honey"]}`)
			rec := reviewBee(url.Values{"name": {tc.name}, "action": {tc.action}})
			assertStatus(t, rec, http.StatusOK)
			assertWrites(t, f, tc.want)
			assertBody(t, rec, ">honey<", "autofocus")
		})
	}
}

func TestBeeReviewRefusesWhatCannotBeSent(t *testing.T) {
	cases := []struct {
		name string
		form url.Values
		want string
	}{
		{"dot", url.Values{"name": {"."}, "action": {"accept"}}, "That name cannot be reviewed here"},
		{"dots", url.Values{"name": {".."}, "action": {"reject"}}, "That name cannot be reviewed here"},
		{"blank", url.Values{"name": {"  "}, "action": {"accept"}}, "That name cannot be reviewed here"},
		{"no action", url.Values{"name": {"buzz"}}, "Choose accept or reject"},
		{"other action", url.Values{"name": {"buzz"}, "action": {"delete"}}, "Choose accept or reject"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAdmin(t)
			rec := reviewBee(tc.form)
			assertStatus(t, rec, http.StatusBadRequest)
			if got := bannerText(rec); got != tc.want {
				t.Errorf("body = %q, want %q", got, tc.want)
			}
			assertWrites(t, f)
		})
	}
}

func TestBeeReviewRefusedShowsTheAPIMessage(t *testing.T) {
	f := newFakeAdmin(t)
	f.problem("PUT /bee-name-generator/suggestion/buzz", 403, "no")
	rec := reviewBee(url.Values{"name": {"buzz"}, "action": {"accept"}})
	assertStatus(t, rec, http.StatusForbidden)
	if got := bannerText(rec); got != "no" {
		t.Errorf("body = %q", got)
	}
}

func TestBeeReviewReloadFailureIsNotReportedAsAFailedReview(t *testing.T) {
	f := newFakeAdmin(t)
	f.on("DELETE /bee-name-generator/suggestion/buzz", 204, ``)
	f.problem("GET /bee-name-generator/suggestion/100", 500, "down")
	rec := reviewBee(url.Values{"name": {"buzz"}, "action": {"reject"}})
	assertStatus(t, rec, http.StatusInternalServerError)
	if got := bannerText(rec); got != "The change was made, but the page could not be refreshed: down" {
		t.Errorf("body = %q", got)
	}
}

func TestBeeAdminPageIsAShell(t *testing.T) {
	f := newFakeAdmin(t)
	rec := getPage("/project/bee-name-generator/admin")
	assertStatus(t, rec, http.StatusOK)
	assertBody(t, rec, `hx-get="/project/bee-name-generator/admin/suggestions"`, `hx-trigger="load"`, "htmx/v4.0.0/htmx.min.js", `id="admin-error"`)
	assertNoBody(t, rec, "loadBeeSuggestions")
	if len(f.uris()) != 0 {
		t.Errorf("the shell called the API: %v", f.uris())
	}
}

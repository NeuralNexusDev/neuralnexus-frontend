package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil/fakeapi"
)

const (
	beeListCall    = "GET /bee-name-generator/suggestion/100"
	beeNamePath    = "/bee-name-generator/suggestion/"
	beeSuggestions = "/project/bee-name-generator/admin/suggestions"
)

func suggestionsJSON(names ...string) string {
	encoded, err := json.Marshal(struct {
		Suggestions []string `json:"suggestions"`
	}{names})
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func postBeeReview(action, name string) *httptest.ResponseRecorder {
	return serveRequest("POST", beeSuggestions, url.Values{"action": {action}, "name": {name}})
}

func TestIsDotSegment(t *testing.T) {
	t.Run("BE-01_the_names_dot_and_dot_dot_are_dot_segments", func(t *testing.T) {
		for _, name := range []string{".", ".."} {
			t.Run("BE-01_"+name, func(t *testing.T) {
				if !isDotSegment(name) {
					t.Errorf("isDotSegment(%q) = false", name)
				}
			})
		}
	})

	t.Run("BE-02_names_that_only_resemble_a_dot_segment_are_not", func(t *testing.T) {
		for _, name := range []string{"", "...", ".a", "a.", "a..b", " .", ".. ", "./", "%2e%2e"} {
			t.Run(fmt.Sprintf("BE-02_%q", name), func(t *testing.T) {
				if isDotSegment(name) {
					t.Errorf("isDotSegment(%q) = true", name)
				}
			})
		}
	})
}

func TestLoadBeeSuggestions(t *testing.T) {
	const apiPath = beeNamePath + "100"

	t.Run("BE-03_BE-04_returns_the_names_in_order_from_a_bodiless_GET_with_the_limit_in_the_path", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On(beeListCall, 200, suggestionsJSON("Buzz", "Aster", "Buzz"))
		data, err := loadBeeSuggestions(newSession(testutil.NewRequest(testutil.SessionCookie("abc123"))), false)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if !slices.Equal(data.Names, []string{"Buzz", "Aster", "Buzz"}) {
			t.Errorf("Names = %q, want the API's order with the duplicate kept", data.Names)
		}
		calls := f.Calls()
		if len(calls) != 1 || calls[0].Method != "GET" || calls[0].URI != apiPath || calls[0].Body != "" {
			t.Fatalf("calls = %+v, want one bodiless GET %s", calls, apiPath)
		}
		if got := calls[0].Header.Values("Content-Type"); len(got) != 0 {
			t.Errorf("Content-Type = %q, want none", got)
		}
	})

	t.Run("BE-05_focusList_is_copied_and_does_not_change_Names", func(t *testing.T) {
		for _, focusList := range []bool{false, true} {
			t.Run(fmt.Sprintf("BE-05_focusList_%t", focusList), func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On(beeListCall, 200, suggestionsJSON("Buzz", "Aster", "Buzz"))
				data, err := loadBeeSuggestions(newSession(testutil.NewRequest()), focusList)
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				if data.FocusList != focusList {
					t.Errorf("FocusList = %t, want %t", data.FocusList, focusList)
				}
				if !slices.Equal(data.Names, []string{"Buzz", "Aster", "Buzz"}) {
					t.Errorf("Names = %q", data.Names)
				}
			})
		}
	})

	t.Run("BE-06_a_response_with_no_suggestions_gives_an_empty_Names", func(t *testing.T) {
		bodies := []struct{ name, body string }{
			{"empty_array", `{"suggestions":[]}`},
			{"empty_object", `{}`},
			{"null", `{"suggestions":null}`},
		}
		for _, tc := range bodies {
			t.Run("BE-06_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On(beeListCall, 200, tc.body)
				data, err := loadBeeSuggestions(newSession(testutil.NewRequest()), false)
				if err != nil || len(data.Names) != 0 {
					t.Errorf("Names = %q, err = %v, want none and no error", data.Names, err)
				}
			})
		}
	})

	t.Run("BE-07_an_error_status_with_a_detail_returns_that_detail", func(t *testing.T) {
		for _, status := range []int{403, 404, 500} {
			t.Run(fmt.Sprintf("BE-07_%d", status), func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.Problem(beeListCall, status, "Not allowed")
				data, err := loadBeeSuggestions(newSession(testutil.NewRequest()), true)
				assertAPIError(t, err, status, "Not allowed", "GET", apiPath)
				if !reflect.DeepEqual(data, components.BeeSuggestionsData{}) {
					t.Errorf("data = %+v, want the zero value", data)
				}
			})
		}
	})

	t.Run("BE-08_an_error_status_without_a_detail_uses_the_load_failure_message", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On(beeListCall, 500, ``)
		data, err := loadBeeSuggestions(newSession(testutil.NewRequest()), true)
		assertAPIError(t, err, 500, "Failed to load suggestions", "GET", apiPath)
		if !reflect.DeepEqual(data, components.BeeSuggestionsData{}) {
			t.Errorf("data = %+v, want the zero value", data)
		}
	})

	t.Run("BE-09_a_401_returns_the_sign-in_sentinel", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On(beeListCall, 401, ``)
		data, err := loadBeeSuggestions(newSession(testutil.NewRequest()), true)
		if !errors.Is(err, errUnauthorized) {
			t.Errorf("err = %v, want errUnauthorized", err)
		}
		if !reflect.DeepEqual(data, components.BeeSuggestionsData{}) {
			t.Errorf("data = %+v, want the zero value", data)
		}
	})

	t.Run("BE-10_a_200_body_that_does_not_decode_gives_a_502", func(t *testing.T) {
		bodies := []struct{ name, body string }{
			{"not_json", "not json"},
			{"wrong_type", `{"suggestions":"x"}`},
		}
		for _, tc := range bodies {
			t.Run("BE-10_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On(beeListCall, 200, tc.body)
				data, err := loadBeeSuggestions(newSession(testutil.NewRequest()), true)
				assertAPIError(t, err, 502, "Failed to load suggestions", "GET", apiPath)
				if !reflect.DeepEqual(data, components.BeeSuggestionsData{}) {
					t.Errorf("data = %+v, want the zero value", data)
				}
			})
		}
	})
}

func TestBeeSuggestionsHandler(t *testing.T) {
	t.Run("BE-11_GET_lists_the_pending_suggestions", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On(beeListCall, 200, suggestionsJSON("Buzz", "Aster"))
		rec := serveRequest("GET", beeSuggestions, nil)
		assertStatusCode(t, rec, 200)
		if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Errorf("Content-Type = %q", got)
		}
		assertBodyHas(t, rec, "Buzz", "Aster")
		assertBodyLacks(t, rec, "autofocus")
		if body := rec.Body.String(); strings.Index(body, "Buzz") > strings.Index(body, "Aster") {
			t.Error("Aster is listed before Buzz")
		}
		if got := f.Lines(); !slices.Equal(got, []string{beeListCall}) {
			t.Errorf("API calls = %q, want one list GET", got)
		}
	})

	t.Run("BE-12_an_empty_list_shows_the_empty_message", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On(beeListCall, 200, suggestionsJSON())
		rec := serveRequest("GET", beeSuggestions, nil)
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, "No pending suggestions")
	})

	t.Run("BE-13_an_API_failure_becomes_an_error_response_with_its_status_and_message", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			detail  string
			message string
		}{
			{"403", 403, "Not allowed", "Not allowed"},
			{"404", 404, "Not found", "Not found"},
			{"500_without_a_detail", 500, "", "Failed to load suggestions"},
		}
		for _, tc := range cases {
			t.Run("BE-13_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				if tc.detail == "" {
					f.On(beeListCall, tc.status, ``)
				} else {
					f.Problem(beeListCall, tc.status, tc.detail)
				}
				rec := serveRequest("GET", beeSuggestions, nil)
				assertStatusCode(t, rec, tc.status)
				if got := rec.Body.String(); got != tc.message {
					t.Errorf("body = %q, want exactly %q", got, tc.message)
				}
				assertBodyLacks(t, rec, `id="bee-suggestions`)
			})
		}
	})

	t.Run("BE-14_a_401_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On(beeListCall, 401, ``)
		assertLoginRedirect(t, serveRequest("GET", beeSuggestions, nil))
	})

	t.Run("BE-15_a_200_body_that_does_not_decode_gives_a_502", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On(beeListCall, 200, "not json")
		rec := serveRequest("GET", beeSuggestions, nil)
		assertStatusCode(t, rec, 502)
		if got := rec.Body.String(); got != "Failed to load suggestions" {
			t.Errorf("body = %q", got)
		}
	})
}

func TestBeeReviewHandler(t *testing.T) {
	const changed = "The change was made, but the page could not be refreshed: "
	actions := []struct{ action, method string }{{"accept", "PUT"}, {"reject", "DELETE"}}

	t.Run("BE-16_accept_sends_a_PUT_for_the_name_then_reloads_the_list", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("PUT "+beeNamePath+"Buzz", 200, `{}`)
		f.On(beeListCall, 200, suggestionsJSON("Aster"))
		rec := postBeeReview("accept", "Buzz")
		assertStatusCode(t, rec, 200)
		if got := f.Lines(); !slices.Equal(got, []string{"PUT " + beeNamePath + "Buzz", beeListCall}) {
			t.Errorf("API calls = %q, want the PUT without a body, then the list GET", got)
		}
		if got := f.Calls()[0].Header.Values("Content-Type"); len(got) != 0 {
			t.Errorf("Content-Type = %q, want none", got)
		}
		assertBodyHas(t, rec, "Aster")
	})

	t.Run("BE-17_reject_sends_a_DELETE_for_the_name_then_reloads_the_list", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("DELETE "+beeNamePath+"Buzz", 200, `{}`)
		f.On(beeListCall, 200, suggestionsJSON("Aster"))
		rec := postBeeReview("reject", "Buzz")
		assertStatusCode(t, rec, 200)
		if got := f.Lines(); !slices.Equal(got, []string{"DELETE " + beeNamePath + "Buzz", beeListCall}) {
			t.Errorf("API calls = %q, want the DELETE without a body, then the list GET", got)
		}
		if got := f.Calls()[0].Header.Values("Content-Type"); len(got) != 0 {
			t.Errorf("Content-Type = %q, want none", got)
		}
		assertBodyHas(t, rec, "Aster")
	})

	t.Run("BE-18_the_refreshed_list_comes_from_the_reload_and_asks_for_focus", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("PUT "+beeNamePath+"Buzz", 200, `{}`)
		f.On(beeListCall, 200, suggestionsJSON("Aster", "Wasp"))
		rec := postBeeReview("accept", "Buzz")
		assertBodyHas(t, rec, "Aster", "Wasp", "autofocus")
		assertBodyLacks(t, rec, "Buzz")
	})

	t.Run("BE-19_a_write_answered_without_a_JSON_object_still_counts_as_success", func(t *testing.T) {
		answers := []struct {
			name   string
			status int
			body   string
		}{
			{"204_no_body", 204, ``},
			{"200_other_object", 200, `{"ok":true}`},
			{"200_empty_object", 200, `{}`},
		}
		for _, tc := range answers {
			t.Run("BE-19_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("PUT "+beeNamePath+"Buzz", tc.status, tc.body)
				f.On(beeListCall, 200, suggestionsJSON("Aster"))
				rec := postBeeReview("accept", "Buzz")
				assertStatusCode(t, rec, 200)
				if got := f.Lines(); !slices.Equal(got, []string{"PUT " + beeNamePath + "Buzz", beeListCall}) {
					t.Errorf("API calls = %q, want the PUT then the reload", got)
				}
			})
		}
	})

	t.Run("BE-20_a_name_is_path-escaped_before_it_goes_into_the_API_path", func(t *testing.T) {
		names := []struct{ name, escaped string }{
			{"a/b", "a%2Fb"},
			{"a?b#c", "a%3Fb%23c"},
			{"100%", "100%25"},
			{"%2e%2e", "%252e%252e"},
			{"a/../b", "a%2F..%2Fb"},
			{"Bée 🐝", "B%C3%A9e%20%F0%9F%90%9D"},
			{`\`, "%5C"},
		}
		for _, a := range actions {
			for _, tc := range names {
				t.Run("BE-20_"+a.action+"_"+tc.escaped, func(t *testing.T) {
					f := fakeapi.NewFakeAPI(t)
					f.On(a.method+" "+beeNamePath+tc.escaped, 200, `{}`)
					f.On(beeListCall, 200, suggestionsJSON())
					rec := postBeeReview(a.action, tc.name)
					assertStatusCode(t, rec, 200)
					if got := f.Lines(); len(got) == 0 || got[0] != a.method+" "+beeNamePath+tc.escaped {
						t.Errorf("API calls = %q, want %s first", got, a.method+" "+beeNamePath+tc.escaped)
					}
				})
			}
		}
	})

	t.Run("BE-21_a_name_is_sent_as_typed_and_a_lookalike_of_a_dot_segment_is_allowed", func(t *testing.T) {
		names := []struct{ name, escaped string }{
			{" Buzz ", "%20Buzz%20"},
			{"...", "..."},
			{".a", ".a"},
			{"a..b", "a..b"},
			{" .. ", "%20..%20"},
		}
		for _, tc := range names {
			t.Run("BE-21_"+tc.escaped, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("PUT "+beeNamePath+tc.escaped, 200, `{}`)
				f.On(beeListCall, 200, suggestionsJSON())
				rec := postBeeReview("accept", tc.name)
				assertStatusCode(t, rec, 200)
				if got := f.Lines(); len(got) == 0 || got[0] != "PUT "+beeNamePath+tc.escaped {
					t.Errorf("API calls = %q, want a PUT for %s first", got, tc.escaped)
				}
			})
		}
	})

	t.Run("BE-22_the_names_dot_and_dot_dot_are_refused_before_any_API_call", func(t *testing.T) {
		for _, a := range actions {
			for _, name := range []string{".", ".."} {
				t.Run("BE-22_"+a.action+"_"+name, func(t *testing.T) {
					f := fakeapi.NewFakeAPI(t)
					assertFailure(t, postBeeReview(a.action, name), 400, "That name cannot be reviewed here")
					if got := f.Lines(); len(got) != 0 {
						t.Errorf("API calls = %q, want none", got)
					}
				})
			}
		}
	})

	t.Run("BE-23_a_missing_empty_or_blank_name_is_refused_before_any_API_call", func(t *testing.T) {
		forms := []struct {
			name string
			form url.Values
		}{
			{"absent", url.Values{"action": {"accept"}}},
			{"empty", url.Values{"action": {"accept"}, "name": {""}}},
			{"spaces", url.Values{"action": {"accept"}, "name": {"  "}}},
			{"tab_and_newline", url.Values{"action": {"accept"}, "name": {"\t\n"}}},
		}
		for _, tc := range forms {
			t.Run("BE-23_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				assertFailure(t, serveRequest("POST", beeSuggestions, tc.form), 400, "That name cannot be reviewed here")
				if got := f.Lines(); len(got) != 0 {
					t.Errorf("API calls = %q, want none", got)
				}
			})
		}
	})

	t.Run("BE-24_an_action_other_than_accept_or_reject_is_refused_before_any_API_call", func(t *testing.T) {
		forms := []struct {
			name string
			form url.Values
		}{
			{"absent", url.Values{"name": {"Buzz"}}},
			{"empty", url.Values{"name": {"Buzz"}, "action": {""}}},
			{"Accept", url.Values{"name": {"Buzz"}, "action": {"Accept"}}},
			{"ACCEPT", url.Values{"name": {"Buzz"}, "action": {"ACCEPT"}}},
			{"delete", url.Values{"name": {"Buzz"}, "action": {"delete"}}},
			{"trailing_space", url.Values{"name": {"Buzz"}, "action": {"accept "}}},
		}
		for _, tc := range forms {
			t.Run("BE-24_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				assertFailure(t, serveRequest("POST", beeSuggestions, tc.form), 400, "Choose accept or reject")
				if got := f.Lines(); len(got) != 0 {
					t.Errorf("API calls = %q, want none", got)
				}
			})
		}
	})

	t.Run("BE-25_a_bad_action_is_reported_before_a_bad_name", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		assertFailure(t, postBeeReview("delete", ""), 400, "Choose accept or reject")
		if got := f.Lines(); len(got) != 0 {
			t.Errorf("API calls = %q, want none", got)
		}
	})

	t.Run("BE-26_a_refused_write_returns_the_API_status_and_message_and_does_not_reload", func(t *testing.T) {
		answers := []struct {
			status int
			detail string
		}{
			{404, "Suggestion not found"},
			{403, "Not allowed"},
			{500, "database down"},
		}
		for _, a := range actions {
			for _, tc := range answers {
				t.Run(fmt.Sprintf("BE-26_%s_%d", a.action, tc.status), func(t *testing.T) {
					f := fakeapi.NewFakeAPI(t)
					f.Problem(a.method+" "+beeNamePath+"Buzz", tc.status, tc.detail)
					assertFailure(t, postBeeReview(a.action, "Buzz"), tc.status, tc.detail)
					if got := f.Lines(); !slices.Equal(got, []string{a.method + " " + beeNamePath + "Buzz"}) {
						t.Errorf("API calls = %q, want only the write", got)
					}
				})
			}
		}
	})

	t.Run("BE-27_a_refused_write_without_a_detail_uses_the_message_for_the_action", func(t *testing.T) {
		answers := []struct {
			name   string
			status int
			body   string
		}{
			{"500_empty_body", 500, ``},
			{"404_html_body", 404, `<html>Not Found</html>`},
		}
		for _, a := range actions {
			for _, tc := range answers {
				t.Run("BE-27_"+a.action+"_"+tc.name, func(t *testing.T) {
					f := fakeapi.NewFakeAPI(t)
					f.On(a.method+" "+beeNamePath+"Buzz", tc.status, tc.body)
					assertFailure(t, postBeeReview(a.action, "Buzz"), tc.status, "Failed to "+a.action+" the suggestion")
					if got := f.Lines(); !slices.Equal(got, []string{a.method + " " + beeNamePath + "Buzz"}) {
						t.Errorf("API calls = %q, want only the write", got)
					}
				})
			}
		}
	})

	t.Run("BE-28_a_401_on_the_write_sends_the_browser_to_the_login_page_and_does_not_reload", func(t *testing.T) {
		for _, a := range actions {
			t.Run("BE-28_"+a.action, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On(a.method+" "+beeNamePath+"Buzz", 401, ``)
				assertLoginRedirect(t, postBeeReview(a.action, "Buzz"))
				if got := f.Lines(); !slices.Equal(got, []string{a.method + " " + beeNamePath + "Buzz"}) {
					t.Errorf("API calls = %q, want only the write", got)
				}
			})
		}
	})

	t.Run("BE-29_a_failed_reload_after_a_write_says_the_change_was_made", func(t *testing.T) {
		answers := []struct {
			name    string
			status  int
			detail  string
			body    string
			message string
		}{
			{"500_with_a_detail", 500, "database down", "", changed + "database down"},
			{"404_with_a_detail", 404, "Not found", "", changed + "Not found"},
			{"500_without_a_detail", 500, "", "", changed + "Failed to load suggestions"},
			{"200_not_json", 200, "", "not json", changed + "Failed to load suggestions"},
		}
		for _, tc := range answers {
			t.Run("BE-29_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("PUT "+beeNamePath+"Buzz", 200, `{}`)
				if tc.detail != "" {
					f.Problem(beeListCall, tc.status, tc.detail)
				} else {
					f.On(beeListCall, tc.status, tc.body)
				}
				status := tc.status
				if tc.status == 200 {
					status = 502
				}
				assertFailure(t, postBeeReview("accept", "Buzz"), status, tc.message)
				if got := f.Lines(); !slices.Equal(got, []string{"PUT " + beeNamePath + "Buzz", beeListCall}) {
					t.Errorf("API calls = %q, want the write once and the reload once", got)
				}
			})
		}
	})

	t.Run("BE-30_a_401_on_the_reload_sends_the_browser_to_the_login_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		f.On("DELETE "+beeNamePath+"Buzz", 200, `{}`)
		f.On(beeListCall, 401, ``)
		rec := postBeeReview("reject", "Buzz")
		assertLoginRedirect(t, rec)
		assertBodyLacks(t, rec, "The change was made")
	})
}

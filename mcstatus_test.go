package main

import (
	"context"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil/fakeapi"
)

const mcSiteURL = "http://site.test"

func assertPlainMcStatusPage(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	assertStatusCode(t, rec, 200)
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	assertBodyHas(t, rec, `<title>NeuralNexus</title>`, `id="mc-status-host"`)
	assertBodyLacks(t, rec, "og:title", `rel="canonical"`)
}

func mcStatusAnswer(t testing.TB, host string, port int, motd, version string) string {
	t.Helper()
	return testutil.JSONString(t, apiMcStatus{Host: host, Port: port, Motd: motd, NumPlayers: 3, MaxPlayers: 20, Version: version})
}

func mcAnswerOfSize(size int) string {
	const prefix, suffix = `{"host":"a.example","port":25565,"motd":"`, `"}`
	return prefix + strings.Repeat("a", size-len(prefix)-len(suffix)) + suffix
}

func metaContent(t testing.TB, body, key string) string {
	t.Helper()
	found := regexp.MustCompile(`<meta (?:property|name)="` + regexp.QuoteMeta(key) + `" content="([^"]*)"`).FindStringSubmatch(body)
	if found == nil {
		t.Errorf("no meta tag for %q in:\n%s", key, body)
		return ""
	}
	return html.UnescapeString(found[1])
}

func TestDecodeMcStatus(t *testing.T) {
	decode := func(body string) (apiMcStatus, bool) { return decodeMcStatus(strings.NewReader(body)) }

	t.Run("MC-01_a_complete_answer_decodes_into_its_fields", func(t *testing.T) {
		got, ok := decode(`{"host":"play.example.net","port":25565,"motd":"Hi","num_players":3,"max_players":20,"version":"1.21"}`)
		want := apiMcStatus{Host: "play.example.net", Port: 25565, Motd: "Hi", NumPlayers: 3, MaxPlayers: 20, Version: "1.21"}
		if !ok || got != want {
			t.Errorf("decode = %+v, %t, want %+v, true", got, ok, want)
		}
	})

	t.Run("MC-02_an_answer_without_a_host_is_refused", func(t *testing.T) {
		for name, body := range map[string]string{"missing": `{"port":25565}`, "empty": `{"host":"","port":25565}`} {
			t.Run("MC-02_"+name, func(t *testing.T) {
				if _, ok := decode(body); ok {
					t.Errorf("decode(%s) ok = true", body)
				}
			})
		}
	})

	t.Run("MC-03_an_answer_without_a_port_or_with_port_0_is_refused", func(t *testing.T) {
		for name, body := range map[string]string{"missing": `{"host":"a.example"}`, "zero": `{"host":"a.example","port":0}`} {
			t.Run("MC-03_"+name, func(t *testing.T) {
				if _, ok := decode(body); ok {
					t.Errorf("decode(%s) ok = true", body)
				}
			})
		}
	})

	t.Run("MC-04_a_body_that_does_not_decode_into_an_answer_is_refused", func(t *testing.T) {
		cases := []struct{ name, body string }{
			{"empty", ``},
			{"not_json", `not json`},
			{"cut_short", `{"host":"a.example","port":25565`},
			{"array", `[]`},
			{"string", `"text"`},
			{"null", `null`},
			{"empty_object", `{}`},
			{"port_as_string", `{"host":"a.example","port":"25565"}`},
			{"players_as_string", `{"host":"a.example","port":25565,"num_players":"3"}`},
			{"host_as_number", `{"host":7,"port":25565}`},
		}
		for _, tc := range cases {
			t.Run("MC-04_"+tc.name, func(t *testing.T) {
				if _, ok := decode(tc.body); ok {
					t.Errorf("decode(%s) ok = true", tc.body)
				}
			})
		}
	})

	t.Run("MC-05_only_the_host_and_port_are_required", func(t *testing.T) {
		got, ok := decode(`{"host":"a.example","port":25565}`)
		if want := (apiMcStatus{Host: "a.example", Port: 25565}); !ok || got != want {
			t.Errorf("decode = %+v, %t, want %+v, true", got, ok, want)
		}
	})

	t.Run("MC-06_unknown_fields_and_data_after_the_first_JSON_value_are_ignored", func(t *testing.T) {
		for name, body := range map[string]string{
			"unknown_field": `{"host":"a.example","port":25565,"extra":{"x":1}}`,
			"trailing_text": `{"host":"a.example","port":25565} trailing text`,
		} {
			t.Run("MC-06_"+name, func(t *testing.T) {
				got, ok := decode(body)
				if !ok || got.Host != "a.example" || got.Port != 25565 {
					t.Errorf("decode = %+v, %t, want a.example and 25565", got, ok)
				}
			})
		}
	})

	t.Run("MC-07_only_the_first_1_MiB_of_the_body_is_read", func(t *testing.T) {
		cases := []struct {
			name string
			body string
			ok   bool
		}{
			{"exactly_the_limit", mcAnswerOfSize(1 << 20), true},
			{"one_byte_over", mcAnswerOfSize(1<<20 + 1), false},
			{"spaces_fill_the_limit", strings.Repeat(" ", 1<<20) + `{"host":"a.example","port":25565}`, false},
		}
		for _, tc := range cases {
			t.Run("MC-07_"+tc.name, func(t *testing.T) {
				got, ok := decode(tc.body)
				if ok != tc.ok {
					t.Fatalf("ok = %t, want %t", ok, tc.ok)
				}
				if ok && (got.Host != "a.example" || got.Port != 25565) {
					t.Errorf("decode = %+v, want a.example and 25565", got)
				}
			})
		}
	})
}

func TestMcStatusPageHandler(t *testing.T) {
	const host = "play.example.net"
	const path = "/project/mc-status/" + host

	online := func(t *testing.T) string { return mcStatusAnswer(t, host, 25565, "A Minecraft Server", "Paper 1.21.4") }
	served := func(t *testing.T, status int, body string) *fakeapi.FakeAPI {
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /mcstatus/"+host, status, body)
		return f
	}
	lookupURI := func(t *testing.T, query string) string {
		f := served(t, 200, online(t))
		serveRequest("GET", path+query, nil)
		calls := f.Calls()
		if len(calls) != 1 {
			t.Fatalf("API calls = %q, want one", f.Lines())
		}
		return calls[0].URI
	}

	t.Run("MC-08_a_request_without_a_host_serves_the_plain_checker_page_and_does_not_call_the_API", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		rec := serveRequest("GET", "/project/mc-status", nil)
		assertStatusCode(t, rec, 200)
		assertHTMLContent(t, rec)
		assertBodyHas(t, rec, `<title>NeuralNexus</title>`, `id="mc-status-host"`)
		assertBodyLacks(t, rec, "og:title", `rel="canonical"`)
		if got := rec.Header().Get("Cache-Control"); got != "" {
			t.Errorf("Cache-Control = %q, want none", got)
		}
		f.AssertLines(t)
	})

	t.Run("MC-09_a_host_longer_than_260_bytes_is_refused_before_any_API_call", func(t *testing.T) {
		for _, length := range []int{261, 1000} {
			t.Run("MC-09_"+strconv.Itoa(length)+"_bytes", func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				assertPlainMcStatusPage(t, serveRequest("GET", "/project/mc-status/"+strings.Repeat("a", length), nil))
				f.AssertLines(t)
			})
		}
	})

	t.Run("MC-10_a_host_of_exactly_260_bytes_is_looked_up", func(t *testing.T) {
		long := strings.Repeat("a", 260)
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /mcstatus/"+long, 200, mcStatusAnswer(t, "a.example", 25565, "Hi", "1.21"))
		rec := serveRequest("GET", "/project/mc-status/"+long, nil)
		assertStatusCode(t, rec, 200)
		assertBodyHas(t, rec, "og:title")
		f.AssertLines(t, "GET /mcstatus/"+long+"?query=true")
	})

	t.Run("MC-11_the_length_limit_counts_decoded_bytes_not_runes_and_not_encoded_characters", func(t *testing.T) {
		cases := []struct {
			name, path, route string
			looked            bool
		}{
			{"130_e_acute", strings.Repeat("%C3%A9", 130), strings.Repeat("%C3%A9", 130), true},
			{"131_e_acute", strings.Repeat("%C3%A9", 131), "", false},
			{"260_encoded_a", strings.Repeat("%61", 260), strings.Repeat("a", 260), true},
			{"261_encoded_a", strings.Repeat("%61", 261), "", false},
		}
		for _, tc := range cases {
			t.Run("MC-11_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				if tc.looked {
					f.On("GET /mcstatus/"+tc.route, 200, mcStatusAnswer(t, "a.example", 25565, "Hi", "1.21"))
				}
				rec := serveRequest("GET", "/project/mc-status/"+tc.path, nil)
				if !tc.looked {
					assertPlainMcStatusPage(t, rec)
					f.AssertLines(t)
					return
				}
				assertStatusCode(t, rec, 200)
				f.AssertLines(t, "GET /mcstatus/"+tc.route+"?query=true")
			})
		}
	})

	t.Run("MC-12_the_hosts_dot_and_dot_dot_are_refused_before_any_API_call", func(t *testing.T) {
		for name, encoded := range map[string]string{"dot": "%2E", "dot_dot_upper": "%2E%2E", "dot_dot_lower": "%2e%2e"} {
			t.Run("MC-12_"+name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				assertPlainMcStatusPage(t, serveRequest("GET", "/project/mc-status/"+encoded, nil))
				f.AssertLines(t)
			})
		}
	})

	t.Run("MC-13_hosts_that_only_resemble_dot_segments_are_looked_up", func(t *testing.T) {
		cases := []struct{ name, path string }{
			{"three_dots", "..."},
			{"leading_dot", ".a"},
			{"trailing_dot", "a."},
			{"space_dot", "%20."},
			{"dot_space", ".%20"},
			{"encoded_dots_encoded_again", "%252e%252e"},
		}
		for _, tc := range cases {
			t.Run("MC-13_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /mcstatus/"+tc.path, 200, mcStatusAnswer(t, "a.example", 25565, "Hi", "1.21"))
				assertStatusCode(t, serveRequest("GET", "/project/mc-status/"+tc.path, nil), 200)
				f.AssertLines(t, "GET /mcstatus/"+tc.path+"?query=true")
			})
		}
	})

	t.Run("MC-14_the_upstream_request_is_one_bodiless_GET_for_the_host_with_the_default_options", func(t *testing.T) {
		f := served(t, 200, online(t))
		serveRequest("GET", path, nil)
		calls := f.Calls()
		if len(calls) != 1 {
			t.Fatalf("API calls = %q, want one", f.Lines())
		}
		if calls[0].Method != "GET" || calls[0].URI != "/mcstatus/play.example.net?query=true" || calls[0].Body != "" {
			t.Errorf("call = %+v, want a bodiless GET of /mcstatus/play.example.net?query=true", calls[0])
		}
	})

	t.Run("MC-15_the_host_is_path_escaped_into_the_upstream_path_and_cannot_add_to_the_query", func(t *testing.T) {
		cases := []struct{ name, path, route string }{
			{"slash", "a%2Fb", "a%2Fb"},
			{"question_mark", "a%3Fb", "a%3Fb"},
			{"hash", "a%23b", "a%23b"},
			{"percent", "100%25", "100%25"},
			{"space", "a%20b", "a%20b"},
			{"e_acute", "%C3%A9", "%C3%A9"},
			{"invalid_utf8_byte", "%FF", "%FF"},
			{"line_feed", "a%0Ab", "a%0Ab"},
			{"slashes_around_dots", "x%2F..%2Fy", "x%2F..%2Fy"},
			{"semicolon_and_comma", "a%3Bb%2Cc", "a%3Bb%2Cc"},
			{"ampersand_query", "a%26query=false", "a&query=false"},
			{"question_mark_bedrock", "a%3Fbedrock%3Dtrue", "a%3Fbedrock=true"},
			{"host_and_port", "play.example.net:25565", "play.example.net:25565"},
			{"ipv6_and_port", "%5B2001:db8::1%5D:25566", "%5B2001:db8::1%5D:25566"},
		}
		for _, tc := range cases {
			t.Run("MC-15_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /mcstatus/"+tc.route, 200, mcStatusAnswer(t, "a.example", 25565, "Hi", "1.21"))
				serveRequest("GET", "/project/mc-status/"+tc.path, nil)
				f.AssertLines(t, "GET /mcstatus/"+tc.route+"?query=true")
			})
		}
	})

	t.Run("MC-16_the_Bedrock_option_sends_only_bedrock_true", func(t *testing.T) {
		for name, query := range map[string]string{
			"alone":      "?bedrock=true",
			"query_port": "?bedrock=true&query_port=25575",
			"query_off":  "?bedrock=true&query=false",
		} {
			t.Run("MC-16_"+name, func(t *testing.T) {
				if got := lookupURI(t, query); got != "/mcstatus/play.example.net?bedrock=true" {
					t.Errorf("URI = %q", got)
				}
			})
		}
	})

	t.Run("MC-17_turning_the_query_option_off_sends_no_query_string", func(t *testing.T) {
		for name, query := range map[string]string{"alone": "?query=false", "query_port": "?query=false&query_port=25575"} {
			t.Run("MC-17_"+name, func(t *testing.T) {
				if got := lookupURI(t, query); got != "/mcstatus/play.example.net" {
					t.Errorf("URI = %q", got)
				}
			})
		}
	})

	t.Run("MC-18_a_valid_query_port_is_sent_along_with_query_true", func(t *testing.T) {
		cases := []struct{ name, query, want string }{
			{"typical", "?query_port=25575", "?query=true&query_port=25575"},
			{"lowest_with_query_true", "?query=true&query_port=1", "?query=true&query_port=1"},
			{"highest", "?query_port=65535", "?query=true&query_port=65535"},
		}
		for _, tc := range cases {
			t.Run("MC-18_"+tc.name, func(t *testing.T) {
				if got := lookupURI(t, tc.query); got != "/mcstatus/play.example.net"+tc.want {
					t.Errorf("URI = %q", got)
				}
			})
		}
	})

	t.Run("MC-19_a_query_port_that_is_not_a_number_from_1_to_65535_is_dropped", func(t *testing.T) {
		cases := []struct{ name, query string }{
			{"zero", "?query_port=0"},
			{"over_the_range", "?query_port=65536"},
			{"negative", "?query_port=-1"},
			{"plus_sign", "?query_port=%2B25575"},
			{"letters", "?query_port=abc"},
			{"empty", "?query_port="},
			{"decimal", "?query_port=1.5"},
			{"leading_space", "?query_port=%2025575"},
			{"overflow", "?query_port=99999999999999999999"},
		}
		for _, tc := range cases {
			t.Run("MC-19_"+tc.name, func(t *testing.T) {
				f := served(t, 200, online(t))
				assertStatusCode(t, serveRequest("GET", path+tc.query, nil), 200)
				f.AssertLines(t, "GET /mcstatus/play.example.net?query=true")
			})
		}
	})

	t.Run("MC-20_only_the_exact_values_bedrock_true_and_query_false_change_the_options_and_the_first_value_of_a_repeated_parameter_wins", func(t *testing.T) {
		cases := []struct{ name, query, want string }{
			{"bedrock_capitalised", "?bedrock=True", "?query=true"},
			{"bedrock_one", "?bedrock=1", "?query=true"},
			{"bedrock_false", "?bedrock=false", "?query=true"},
			{"bedrock_empty", "?bedrock=", "?query=true"},
			{"query_capitalised", "?query=False", "?query=true"},
			{"query_zero", "?query=0", "?query=true"},
			{"query_empty", "?query=", "?query=true"},
			{"bedrock_false_first", "?bedrock=false&bedrock=true", "?query=true"},
			{"bedrock_true_first", "?bedrock=true&bedrock=false", "?bedrock=true"},
		}
		for _, tc := range cases {
			t.Run("MC-20_"+tc.name, func(t *testing.T) {
				if got := lookupURI(t, tc.query); got != "/mcstatus/play.example.net"+tc.want {
					t.Errorf("URI = %q", got)
				}
			})
		}
	})

	t.Run("MC-21_other_query_parameters_and_the_visitors_credentials_are_not_sent_to_the_API", func(t *testing.T) {
		f := served(t, 200, online(t))
		req := httptest.NewRequest("GET", path+"?foo=bar&host=evil.example&icon=1", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})
		req.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})
		req.Header.Set("Authorization", "Bearer t")
		NewWebServer("", false).Setup().ServeHTTP(httptest.NewRecorder(), req)
		calls := f.Calls()
		if len(calls) != 1 {
			t.Fatalf("API calls = %q, want one", f.Lines())
		}
		if calls[0].URI != "/mcstatus/play.example.net?query=true" {
			t.Errorf("URI = %q", calls[0].URI)
		}
		for _, header := range []string{"Cookie", "Authorization"} {
			if got := calls[0].Header.Values(header); len(got) != 0 {
				t.Errorf("the API saw %s = %q, want none", header, got)
			}
		}
	})

	t.Run("MC-22_an_online_answer_renders_the_preview_page_with_a_one_minute_public_cache", func(t *testing.T) {
		fakeapi.PointSiteAt(t, mcSiteURL)
		f := served(t, 200, online(t))
		rec := serveRequest("GET", path, nil)
		assertStatusCode(t, rec, 200)
		assertHTMLContent(t, rec)
		if got := rec.Header().Get("Cache-Control"); got != "public, max-age=60" {
			t.Errorf("Cache-Control = %q, want public, max-age=60", got)
		}
		body := rec.Body.String()
		canonical := mcSiteURL + "/project/mc-status/play.example.net"
		assertBodyHas(t, rec, `<title>play.example.net</title>`, `<link rel="canonical" href="`+canonical+`">`, `name="robots" content="noindex"`)
		for key, want := range map[string]string{
			"og:title":       "play.example.net",
			"og:description": "A Minecraft Server\nPlayers: 3/20\nVersion: Paper 1.21.4",
			"og:url":         canonical,
			"og:image":       f.URL + "/api/v1/mcstatus/icon/play.example.net",
		} {
			if got := metaContent(t, body, key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
	})

	t.Run("MC-23_the_page_shows_the_host_and_port_from_the_answer_not_the_path_value", func(t *testing.T) {
		fakeapi.PointSiteAt(t, mcSiteURL)
		f := fakeapi.NewFakeAPI(t)
		f.On("GET /mcstatus/PLAY.Example.net", 200, mcStatusAnswer(t, host, 25566, "Hi", "1.21"))
		rec := serveRequest("GET", "/project/mc-status/PLAY.Example.net", nil)
		assertBodyHas(t, rec, `<title>play.example.net:25566</title>`, `href="`+mcSiteURL+`/project/mc-status/play.example.net:25566"`)
		assertBodyLacks(t, rec, "PLAY")
	})

	t.Run("MC-24_the_request_options_appear_in_the_canonical_URL_and_icon_URL_and_only_one_share_option_is_kept", func(t *testing.T) {
		cases := []struct {
			name, query string
			port        int
			share, icon string
		}{
			{"no_parameter", "", 25565, "", ""},
			{"bedrock", "?bedrock=true", 19132, "?bedrock=true", "?bedrock=true"},
			{"query_off", "?query=false", 25565, "?query=false", ""},
			{"query_port", "?query_port=25575", 25565, "?query_port=25575", ""},
			{"query_off_and_port", "?query=false&query_port=25575", 25565, "?query=false", ""},
			{"bedrock_and_query_off_and_port", "?bedrock=true&query=false&query_port=25575", 19132, "?bedrock=true", "?bedrock=true"},
		}
		for _, tc := range cases {
			t.Run("MC-24_"+tc.name, func(t *testing.T) {
				fakeapi.PointSiteAt(t, mcSiteURL)
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /mcstatus/h.example", 200, mcStatusAnswer(t, "h.example", tc.port, "Hi", "1.21"))
				rec := serveRequest("GET", "/project/mc-status/h.example"+tc.query, nil)
				canonical := mcSiteURL + "/project/mc-status/h.example" + tc.share
				assertBodyHas(t, rec, `<link rel="canonical" href="`+canonical+`">`)
				if got := metaContent(t, rec.Body.String(), "og:url"); got != canonical {
					t.Errorf("og:url = %q, want %q", got, canonical)
				}
				if got, want := metaContent(t, rec.Body.String(), "og:image"), f.URL+"/api/v1/mcstatus/icon/h.example"+tc.icon; got != want {
					t.Errorf("og:image = %q, want %q", got, want)
				}
			})
		}
	})

	motdCases := func(t *testing.T, row string, cases []struct{ name, motd, want string }) {
		for _, tc := range cases {
			t.Run(row+"_"+tc.name, func(t *testing.T) {
				served(t, 200, mcStatusAnswer(t, host, 25565, tc.motd, "Paper 1.21.4"))
				rec := serveRequest("GET", path, nil)
				assertStatusCode(t, rec, 200)
				if got := metaContent(t, rec.Body.String(), "og:description"); got != tc.want {
					t.Errorf("og:description = %q, want %q", got, tc.want)
				}
			})
		}
	}
	const tail = "Players: 3/20\nVersion: Paper 1.21.4"

	t.Run("MC-25_the_MOTD_is_split_into_trimmed_lines_and_blank_lines_are_dropped", func(t *testing.T) {
		motdCases(t, "MC-25", []struct{ name, motd, want string }{
			{"json_newline", "A\nB", "A\nB\n" + tail},
			{"backslash_n", "A\\nB", "A\nB\n" + tail},
			{"padding_and_blank_lines", "  A  \r\n   \n\tB\t", "A\nB\n" + tail},
			{"backslash_n_then_newline", "A\\n\nB", "A\nB\n" + tail},
		})
	})

	t.Run("MC-26_section_sign_color_codes_are_removed_from_the_MOTD", func(t *testing.T) {
		motdCases(t, "MC-26", []struct{ name, motd, want string }{
			{"green_bold", "§aGreen §lBold§r", "Green Bold\n" + tail},
			{"gold_gray", "§6§lGold §7Gray", "Gold Gray\n" + tail},
			{"two_lines", "§4Red\n§9Blue", "Red\nBlue\n" + tail},
		})
	})

	t.Run("MC-27_a_color_code_takes_the_character_after_it_including_a_newline_and_a_final_section_sign_stays", func(t *testing.T) {
		motdCases(t, "MC-27", []struct{ name, motd, want string }{
			{"final_section_sign", "A§", "A§\n" + tail},
			{"section_sign_code", "§§aX", "aX\n" + tail},
			{"code_takes_the_newline", "A§\nB", "AB\n" + tail},
			{"code_takes_a_two_byte_rune", "§éX", "X\n" + tail},
			{"code_takes_a_four_byte_rune", "§😀X", "X\n" + tail},
		})
	})

	t.Run("MC-28_a_MOTD_with_no_visible_text_adds_no_MOTD_line", func(t *testing.T) {
		motdCases(t, "MC-28", []struct{ name, motd, want string }{
			{"empty", "", tail},
			{"spaces", "   ", tail},
			{"blank_lines", "\n\t\n", tail},
			{"only_codes", "§a§l", tail},
		})
	})

	t.Run("MC-29_a_long_MOTD_and_version_are_cut_in_the_page", func(t *testing.T) {
		served(t, 200, mcStatusAnswer(t, host, 25565, strings.Repeat("x", 300), strings.Repeat("v", 100)))
		rec := serveRequest("GET", path, nil)
		want := strings.Repeat("x", 200) + "…\nPlayers: 3/20\nVersion: " + strings.Repeat("v", 64) + "…"
		if got := metaContent(t, rec.Body.String(), "og:description"); got != want {
			t.Errorf("og:description = %q, want %q", got, want)
		}
	})

	t.Run("MC-30_server_supplied_MOTD_and_version_text_is_escaped_in_the_page", func(t *testing.T) {
		cases := []struct {
			name, motd, version string
			want                []string
		}{
			{"script_in_motd_and_version", `"><script>alert(1)</script>`, `"><script>alert(2)</script>`,
				[]string{`&#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;`, `&#34;&gt;&lt;script&gt;alert(2)&lt;/script&gt;`}},
			{"image_tag", `<img src=x onerror=alert(1)>`, "1.21", []string{`&lt;img src=x onerror=alert(1)&gt;`}},
			{"ampersand_and_quotes", `&amp; ' "`, "1.21", []string{`&amp;amp; &#39; &#34;`}},
		}
		for _, tc := range cases {
			t.Run("MC-30_"+tc.name, func(t *testing.T) {
				served(t, 200, mcStatusAnswer(t, host, 25565, tc.motd, tc.version))
				rec := serveRequest("GET", path, nil)
				assertBodyHas(t, rec, tc.want...)
				assertBodyLacks(t, rec, "<script>alert(", "<img src=x", `"><script>`)
			})
		}
	})

	t.Run("MC-31_a_host_from_the_answer_is_escaped_in_the_title_the_tags_and_the_URLs_for_online_and_offline_answers", func(t *testing.T) {
		const hostile = `evil"><script>alert(3)</script>`
		const shown = `evil&#34;&gt;&lt;script&gt;alert(3)&lt;/script&gt;`
		for name, status := range map[string]int{"online": 200, "offline": 404} {
			t.Run("MC-31_"+name, func(t *testing.T) {
				fakeapi.PointSiteAt(t, mcSiteURL)
				served(t, status, mcStatusAnswer(t, hostile, 25565, "Hi", "1.21"))
				rec := serveRequest("GET", path, nil)
				assertBodyHas(t, rec, `<title>`+shown+`</title>`, `property="og:title" content="`+shown+`"`, `property="og:image:alt" content="`+shown+` server icon"`)
				body := rec.Body.String()
				for _, value := range []string{metaContent(t, body, "og:url"), metaContent(t, body, "og:image")} {
					if !strings.Contains(value, "evil%22%3E%3Cscript%3E") {
						t.Errorf("URL %q lacks the escaped host", value)
					}
				}
				if !strings.Contains(body, `rel="canonical" href="`+mcSiteURL+`/project/mc-status/evil%22%3E%3Cscript%3E`) {
					t.Errorf("the canonical link lacks the escaped host:\n%s", body)
				}
				assertBodyLacks(t, rec, "<script>alert(3)", `evil"><`)
			})
		}
	})

	offlineBody := `{"host":"play.example.net","port":25565}`

	t.Run("MC-32_a_404_problem_json_answer_renders_the_offline_page_with_a_30_second_public_cache", func(t *testing.T) {
		fakeapi.PointSiteAt(t, mcSiteURL)
		f := served(t, 404, offlineBody)
		rec := serveRequest("GET", path, nil)
		assertStatusCode(t, rec, 200)
		if got := rec.Header().Get("Cache-Control"); got != "public, max-age=30" {
			t.Errorf("Cache-Control = %q, want public, max-age=30", got)
		}
		assertBodyHas(t, rec, `<title>play.example.net</title>`, `href="`+mcSiteURL+`/project/mc-status/play.example.net"`)
		body := rec.Body.String()
		if got := metaContent(t, body, "og:description"); got != "Server offline or unreachable" {
			t.Errorf("og:description = %q", got)
		}
		if got, want := metaContent(t, body, "og:image"), f.URL+"/api/v1/mcstatus/icon/play.example.net"; got != want {
			t.Errorf("og:image = %q, want %q", got, want)
		}
		assertBodyLacks(t, rec, "Players:")
	})

	t.Run("MC-33_the_problem_json_media_type_is_matched_without_regard_to_case_or_parameters", func(t *testing.T) {
		for name, contentType := range map[string]string{"with_a_parameter": "application/problem+json; charset=utf-8", "upper_case": "APPLICATION/PROBLEM+JSON"} {
			t.Run("MC-33_"+name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.OnType("GET /mcstatus/"+host, 404, contentType, offlineBody)
				rec := serveRequest("GET", path, nil)
				assertStatusCode(t, rec, 200)
				if got := rec.Header().Get("Cache-Control"); got != "public, max-age=30" {
					t.Errorf("Cache-Control = %q, want public, max-age=30", got)
				}
				assertBodyHas(t, rec, `<title>play.example.net</title>`, "Server offline or unreachable")
			})
		}
	})

	t.Run("MC-34_a_404_that_is_not_problem_json_gets_the_plain_page", func(t *testing.T) {
		cases := []struct{ name, contentType string }{
			{"json", "application/json"},
			{"html", "text/html"},
			{"problem_xml", "application/problem+xml"},
			{"problem_jsonx", "application/problem+jsonx"},
			{"no_content_type", ""},
		}
		for _, tc := range cases {
			t.Run("MC-34_"+tc.name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				f.OnType("GET /mcstatus/"+host, 404, tc.contentType, offlineBody)
				assertPlainMcStatusPage(t, serveRequest("GET", path, nil))
			})
		}
	})

	t.Run("MC-35_a_404_problem_json_answer_without_a_usable_host_and_port_gets_the_plain_page", func(t *testing.T) {
		cases := []struct{ name, body string }{
			{"detail_only", `{"detail":"Not found"}`},
			{"host_only", `{"host":"a.example"}`},
			{"port_only", `{"port":25565}`},
			{"empty", ``},
			{"not_json", `not json`},
		}
		for _, tc := range cases {
			t.Run("MC-35_"+tc.name, func(t *testing.T) {
				served(t, 404, tc.body)
				assertPlainMcStatusPage(t, serveRequest("GET", path, nil))
			})
		}
	})

	t.Run("MC-36_an_offline_Bedrock_answer_is_shown_without_the_default_port_and_keeps_its_options", func(t *testing.T) {
		cases := []struct {
			name  string
			port  int
			title string
		}{
			{"default_port", 19132, "h.example"},
			{"other_port", 25565, "h.example:25565"},
		}
		for _, tc := range cases {
			t.Run("MC-36_"+tc.name, func(t *testing.T) {
				fakeapi.PointSiteAt(t, mcSiteURL)
				f := fakeapi.NewFakeAPI(t)
				f.On("GET /mcstatus/h.example", 404, `{"host":"h.example","port":`+strconv.Itoa(tc.port)+`}`)
				rec := serveRequest("GET", "/project/mc-status/h.example?bedrock=true", nil)
				assertBodyHas(t, rec, `<title>`+tc.title+`</title>`)
				if tc.port == 19132 {
					assertBodyHas(t, rec, `href="`+mcSiteURL+`/project/mc-status/h.example?bedrock=true"`)
				}
				if got := rec.Header().Get("Cache-Control"); got != "public, max-age=30" {
					t.Errorf("Cache-Control = %q, want public, max-age=30", got)
				}
			})
		}
	})

	t.Run("MC-37_any_other_API_status_gets_the_plain_page", func(t *testing.T) {
		cases := []struct {
			status int
			body   string
		}{
			{201, ""}, {400, offlineBody}, {401, offlineBody}, {403, offlineBody}, {429, offlineBody}, {500, offlineBody}, {502, offlineBody}, {503, offlineBody},
		}
		for _, tc := range cases {
			t.Run("MC-37_"+strconv.Itoa(tc.status), func(t *testing.T) {
				body := tc.body
				if body == "" {
					body = online(t)
				}
				served(t, tc.status, body)
				assertPlainMcStatusPage(t, serveRequest("GET", path, nil))
			})
		}
	})

	t.Run("MC-38_a_200_answer_that_does_not_decode_into_a_host_and_port_gets_the_plain_page", func(t *testing.T) {
		cases := []struct{ name, body string }{
			{"empty", ``},
			{"not_json", `not json`},
			{"empty_object", `{}`},
			{"host_only", `{"host":"a.example"}`},
			{"port_only", `{"port":25565}`},
			{"port_as_text", `{"host":"a.example","port":"x"}`},
			{"array", `[]`},
			{"null", `null`},
		}
		for _, tc := range cases {
			t.Run("MC-38_"+tc.name, func(t *testing.T) {
				served(t, 200, tc.body)
				assertPlainMcStatusPage(t, serveRequest("GET", path, nil))
			})
		}
	})

	t.Run("MC-39_an_answer_larger_than_1_MiB_gets_the_plain_page", func(t *testing.T) {
		for name, status := range map[string]int{"online": 200, "offline": 404} {
			t.Run("MC-39_"+name, func(t *testing.T) {
				served(t, status, mcAnswerOfSize(1<<20+1))
				assertPlainMcStatusPage(t, serveRequest("GET", path, nil))
			})
		}
	})

	t.Run("MC-40_an_unreachable_API_gets_the_plain_page", func(t *testing.T) {
		fakeapi.PointAPIAtClosed(t)
		rec := serveRequest("GET", path, nil)
		assertPlainMcStatusPage(t, rec)
		assertBodyLacks(t, rec, "refused", "dial")
	})

	t.Run("MC-41_an_API_URL_that_cannot_form_a_request_gets_the_plain_page_without_a_call", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		fakeapi.PointAPIAt(t, "http://[::1")
		assertPlainMcStatusPage(t, serveRequest("GET", path, nil))
		f.AssertLines(t)
	})

	t.Run("MC-42_a_visitor_who_disconnects_ends_the_upstream_call_and_the_handler_returns_without_a_preview_page", func(t *testing.T) {
		f := fakeapi.NewFakeAPI(t)
		started := make(chan struct{})
		f.Handle("GET /mcstatus/"+host, testutil.HangHandler(started, nil))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req := httptest.NewRequest("GET", path, nil).WithContext(ctx)
		req.SetPathValue("host", host)
		rec := httptest.NewRecorder()
		done := make(chan struct{})
		go func() {
			defer close(done)
			McStatusPageHandler(rec, req)
		}()
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("the lookup never reached the API")
		}
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("canceling the request did not end the handler")
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", got)
		}
		if strings.Contains(rec.Body.String(), "og:title") {
			t.Errorf("the body holds og:title:\n%s", rec.Body.String())
		}
	})

	t.Run("MC-43_the_API_response_body_is_closed_once_on_every_outcome", func(t *testing.T) {
		cases := []struct {
			name, contentType, body string
			status                  int
		}{
			{"online", "application/json", mcStatusAnswer(t, host, 25565, "Hi", "1.21"), 200},
			{"online_not_json", "application/json", "not json", 200},
			{"offline", "application/problem+json", offlineBody, 404},
			{"not_found_html", "text/html", offlineBody, 404},
			{"server_error", "application/problem+json", `{"detail":"x"}`, 500},
		}
		for _, tc := range cases {
			t.Run("MC-43_"+tc.name, func(t *testing.T) {
				fakeapi.PointAPIAt(t, "http://api.test")
				closes := 0
				previous := http.DefaultClient.Transport
				t.Cleanup(func() { http.DefaultClient.Transport = previous })
				http.DefaultClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: tc.status,
						Header:     http.Header{"Content-Type": {tc.contentType}},
						Body:       &closeCounter{ReadCloser: io.NopCloser(strings.NewReader(tc.body)), closes: &closes},
						Request:    req,
					}, nil
				})
				serveRequest("GET", path, nil)
				if closes != 1 {
					t.Errorf("Close was called %d times, want 1", closes)
				}
			})
		}
	})

	t.Run("MC-44_a_write_method_does_not_reach_the_handler", func(t *testing.T) {
		for _, method := range []string{"POST", "PUT", "DELETE", "PATCH"} {
			t.Run("MC-44_"+method, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				rec := serveRequest(method, path, nil)
				assertStatusCode(t, rec, 200)
				assertBodyHas(t, rec, `id="auth-error"`)
				assertBodyLacks(t, rec, `id="mc-status-host"`)
				f.AssertLines(t)
			})
		}
	})

	t.Run("MC-45_a_host_path_with_an_extra_segment_or_a_trailing_slash_is_not_routed_to_the_handler", func(t *testing.T) {
		for name, target := range map[string]string{"extra_segment": "/project/mc-status/a/b", "trailing_slash": "/project/mc-status/"} {
			t.Run("MC-45_"+name, func(t *testing.T) {
				f := fakeapi.NewFakeAPI(t)
				rec := serveRequest("GET", target, nil)
				assertStatusCode(t, rec, 200)
				assertBodyHas(t, rec, `id="auth-error"`)
				assertBodyLacks(t, rec, `id="mc-status-host"`)
				f.AssertLines(t)
			})
		}
	})
}

func TestMcStatusEmbedHead(t *testing.T) {
	t.Run("MC-68_the_head_holds_every_preview_tag_with_the_values_from_the_data", func(t *testing.T) {
		fakeapi.PointSiteAt(t, mcSiteURL)
		fakeapi.PointAPIAt(t, "http://api.test")
		data := components.McStatusEmbedData{Host: "play.example.net", Port: 25566, Query: true, QueryPort: 25575, Online: true, Motd: []string{"Hi"}, Players: 1, Max: 2, Version: "1.21"}
		var out strings.Builder
		if err := components.McStatusEmbedPage(data).Render(context.Background(), &out); err != nil {
			t.Fatalf("render: %v", err)
		}
		page := out.String()

		const pageURL = "http://site.test/project/mc-status/play.example.net:25566?query_port=25575"
		for _, want := range []string{`<title>play.example.net:25566</title>`, `id="mc-status-host"`} {
			if !strings.Contains(page, want) {
				t.Errorf("the page lacks %q", want)
			}
		}
		tags := []string{
			`<meta property="og:site_name" content="Powered by NeuralNexus.dev">`,
			`<meta property="og:type" content="website">`,
			`<meta name="twitter:card" content="summary">`,
			`<meta name="robots" content="noindex">`,
			`<meta name="theme-color" content="#7C0014">`,
			`<link rel="canonical" href="` + pageURL + `">`,
			`<meta property="og:url" content="` + pageURL + `">`,
			`<meta property="og:title" content="play.example.net:25566">`,
			"<meta property=\"og:description\" content=\"Hi\nPlayers: 1/2\nVersion: 1.21\">",
			`<meta property="og:image" content="http://api.test/api/v1/mcstatus/icon/play.example.net:25566">`,
			`<meta property="og:image:alt" content="play.example.net:25566 server icon">`,
			`<meta property="og:image:width" content="64">`,
			`<meta property="og:image:height" content="64">`,
		}
		for _, tag := range tags {
			if n := strings.Count(page, tag); n != 1 {
				t.Errorf("%q appears %d times, want 1", tag, n)
			}
		}
	})
}

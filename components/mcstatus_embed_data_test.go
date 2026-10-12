package components

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil/fakeapi"
)

const mcSiteURL = "http://site.test"

func TestMcStatusEmbedDataOptions(t *testing.T) {
	check := func(t *testing.T, row string, cases []struct {
		name string
		data McStatusEmbedData
		want string
	}) {
		for _, tc := range cases {
			t.Run(row+"_"+tc.name, func(t *testing.T) {
				if got := tc.data.Options().Encode(); got != tc.want {
					t.Errorf("Options().Encode() = %q, want %q", got, tc.want)
				}
			})
		}
	}
	type options = struct {
		name string
		data McStatusEmbedData
		want string
	}

	t.Run("MC-46_the_default_Java_lookup_asks_for_the_query_protocol_with_the_port_when_one_is_set", func(t *testing.T) {
		check(t, "MC-46", []options{
			{"without_a_port", McStatusEmbedData{Query: true}, "query=true"},
			{"with_a_port", McStatusEmbedData{Query: true, QueryPort: 25575}, "query=true&query_port=25575"},
		})
	})

	t.Run("MC-47_the_Bedrock_option_wins_over_the_query_options", func(t *testing.T) {
		check(t, "MC-47", []options{
			{"with_query_options", McStatusEmbedData{Bedrock: true, Query: true, QueryPort: 25575}, "bedrock=true"},
			{"alone", McStatusEmbedData{Bedrock: true}, "bedrock=true"},
		})
	})

	t.Run("MC-48_with_the_query_option_off_the_lookup_has_no_parameters_even_with_a_query_port", func(t *testing.T) {
		for name, data := range map[string]McStatusEmbedData{"zero_value": {}, "with_a_port": {QueryPort: 25575}} {
			t.Run("MC-48_"+name, func(t *testing.T) {
				if got := data.Options(); len(got) != 0 || got.Encode() != "" {
					t.Errorf("Options() = %v, want empty", got)
				}
			})
		}
	})
}

func TestMcStatusEmbedDataShareOptions(t *testing.T) {
	check := func(t *testing.T, row string, cases []struct {
		name string
		data McStatusEmbedData
		want string
	}) {
		for _, tc := range cases {
			t.Run(row+"_"+tc.name, func(t *testing.T) {
				if got := tc.data.ShareOptions().Encode(); got != tc.want {
					t.Errorf("ShareOptions().Encode() = %q, want %q", got, tc.want)
				}
			})
		}
	}
	type options = struct {
		name string
		data McStatusEmbedData
		want string
	}

	t.Run("MC-49_the_default_lookup_adds_nothing_to_a_shared_URL_and_a_query_port_is_shared", func(t *testing.T) {
		check(t, "MC-49", []options{
			{"default", McStatusEmbedData{Query: true}, ""},
			{"query_port", McStatusEmbedData{Query: true, QueryPort: 25575}, "query_port=25575"},
		})
	})

	t.Run("MC-50_the_Bedrock_option_is_the_only_shared_option_when_it_is_set", func(t *testing.T) {
		check(t, "MC-50", []options{
			{"alone", McStatusEmbedData{Bedrock: true}, "bedrock=true"},
			{"with_query_options", McStatusEmbedData{Bedrock: true, Query: true, QueryPort: 25575}, "bedrock=true"},
			{"with_query_off", McStatusEmbedData{Bedrock: true, Query: false}, "bedrock=true"},
		})
	})

	t.Run("MC-51_turning_the_query_option_off_is_shared_as_query_false_and_hides_the_query_port", func(t *testing.T) {
		check(t, "MC-51", []options{
			{"zero_value", McStatusEmbedData{}, "query=false"},
			{"with_a_port", McStatusEmbedData{QueryPort: 25575}, "query=false"},
		})
	})
}

func TestMcStatusEmbedDataDisplayHost(t *testing.T) {
	check := func(t *testing.T, row string, cases []struct {
		name string
		data McStatusEmbedData
		want string
	}) {
		for _, tc := range cases {
			t.Run(row+"_"+tc.name, func(t *testing.T) {
				if got := tc.data.DisplayHost(); got != tc.want {
					t.Errorf("DisplayHost() = %q, want %q", got, tc.want)
				}
			})
		}
	}
	type display = struct {
		name string
		data McStatusEmbedData
		want string
	}

	t.Run("MC-52_the_port_is_shown_unless_it_is_the_default_of_the_edition", func(t *testing.T) {
		check(t, "MC-52", []display{
			{"java_default", McStatusEmbedData{Host: "play.example.net", Port: 25565}, "play.example.net"},
			{"java_other_port", McStatusEmbedData{Host: "play.example.net", Port: 25566}, "play.example.net:25566"},
			{"java_with_the_bedrock_port", McStatusEmbedData{Host: "play.example.net", Port: 19132}, "play.example.net:19132"},
			{"bedrock_default", McStatusEmbedData{Bedrock: true, Host: "play.example.net", Port: 19132}, "play.example.net"},
			{"bedrock_with_the_java_port", McStatusEmbedData{Bedrock: true, Host: "play.example.net", Port: 25565}, "play.example.net:25565"},
			{"ipv4", McStatusEmbedData{Host: "192.0.2.1", Port: 25565}, "192.0.2.1"},
		})
	})

	t.Run("MC-53_an_IPv6_host_is_put_in_brackets_before_the_port_is_added", func(t *testing.T) {
		check(t, "MC-53", []display{
			{"java_default", McStatusEmbedData{Host: "2001:db8::1", Port: 25565}, "[2001:db8::1]"},
			{"java_other_port", McStatusEmbedData{Host: "2001:db8::1", Port: 25566}, "[2001:db8::1]:25566"},
			{"bedrock_default", McStatusEmbedData{Bedrock: true, Host: "2001:db8::1", Port: 19132}, "[2001:db8::1]"},
		})
	})
}

func TestMcStatusEmbedDataPageURL(t *testing.T) {
	const base = "http://site.test/project/mc-status/"

	t.Run("MC-54_the_page_URL_is_the_site_URL_the_path_escaped_display_host_and_the_share_options", func(t *testing.T) {
		fakeapi.PointSiteAt(t, mcSiteURL)
		cases := []struct {
			name string
			data McStatusEmbedData
			want string
		}{
			{"default", McStatusEmbedData{Host: "play.example.net", Port: 25565, Query: true}, base + "play.example.net"},
			{"port_and_query_port", McStatusEmbedData{Host: "play.example.net", Port: 25566, Query: true, QueryPort: 25575}, base + "play.example.net:25566?query_port=25575"},
			{"bedrock", McStatusEmbedData{Host: "play.example.net", Port: 19132, Bedrock: true, Query: true}, base + "play.example.net?bedrock=true"},
			{"query_off", McStatusEmbedData{Host: "play.example.net", Port: 25565}, base + "play.example.net?query=false"},
		}
		for _, tc := range cases {
			t.Run("MC-54_"+tc.name, func(t *testing.T) {
				if got := tc.data.PageURL(); got != tc.want {
					t.Errorf("PageURL() = %q, want %q", got, tc.want)
				}
			})
		}
	})

	t.Run("MC-55_an_IPv6_host_is_escaped_in_the_page_URL", func(t *testing.T) {
		fakeapi.PointSiteAt(t, mcSiteURL)
		data := McStatusEmbedData{Host: "2001:db8::1", Port: 25566, Query: true}
		if got, want := data.PageURL(), base+"%5B2001:db8::1%5D:25566"; got != want {
			t.Errorf("PageURL() = %q, want %q", got, want)
		}
	})

	t.Run("MC-56_characters_that_could_break_out_of_the_path_are_escaped_in_the_host", func(t *testing.T) {
		fakeapi.PointSiteAt(t, mcSiteURL)
		cases := []struct{ name, host, escaped string }{
			{"slash", "a/b", "a%2Fb"},
			{"question_mark", "a?b", "a%3Fb"},
			{"hash", "a#b", "a%23b"},
			{"percent", "100%", "100%25"},
			{"space", "a b", "a%20b"},
			{"markup", `evil"><script>`, "evil%22%3E%3Cscript%3E"},
		}
		for _, tc := range cases {
			t.Run("MC-56_"+tc.name, func(t *testing.T) {
				got := McStatusEmbedData{Host: tc.host, Port: 25565, Query: true}.PageURL()
				if got != base+tc.escaped {
					t.Errorf("PageURL() = %q, want %q", got, base+tc.escaped)
				}
				if strings.Contains(got, "?") {
					t.Errorf("PageURL() = %q holds a query string", got)
				}
			})
		}
	})
}

func TestMcStatusEmbedDataIconURL(t *testing.T) {
	const base = "http://api.test/api/v1/mcstatus/icon/"

	t.Run("MC-57_the_icon_URL_is_the_API_icon_route_for_the_display_host_with_the_Bedrock_option_only", func(t *testing.T) {
		fakeapi.PointAPIAt(t, "http://api.test")
		cases := []struct {
			name string
			data McStatusEmbedData
			want string
		}{
			{"query_options_are_left_out", McStatusEmbedData{Host: "play.example.net", Port: 25565, Query: true, QueryPort: 25575}, base + "play.example.net"},
			{"port_is_kept", McStatusEmbedData{Host: "play.example.net", Port: 25566, Query: false}, base + "play.example.net:25566"},
			{"bedrock", McStatusEmbedData{Host: "play.example.net", Port: 19132, Bedrock: true}, base + "play.example.net?bedrock=true"},
		}
		for _, tc := range cases {
			t.Run("MC-57_"+tc.name, func(t *testing.T) {
				if got := tc.data.IconURL(); got != tc.want {
					t.Errorf("IconURL() = %q, want %q", got, tc.want)
				}
			})
		}
	})

	t.Run("MC-58_an_IPv6_host_and_characters_that_could_break_out_of_the_path_are_escaped_in_the_icon_URL", func(t *testing.T) {
		fakeapi.PointAPIAt(t, "http://api.test")
		cases := []struct {
			name, host string
			port       int
			escaped    string
		}{
			{"ipv6_with_a_port", "2001:db8::1", 25566, "%5B2001:db8::1%5D:25566"},
			{"slash", "a/b", 25565, "a%2Fb"},
			{"question_mark", "a?b", 25565, "a%3Fb"},
			{"hash", "a#b", 25565, "a%23b"},
			{"markup", `evil"><script>`, 25565, "evil%22%3E%3Cscript%3E"},
		}
		for _, tc := range cases {
			t.Run("MC-58_"+tc.name, func(t *testing.T) {
				got := McStatusEmbedData{Host: tc.host, Port: tc.port}.IconURL()
				if got != base+tc.escaped {
					t.Errorf("IconURL() = %q, want %q", got, base+tc.escaped)
				}
				if strings.Contains(got, "?") {
					t.Errorf("IconURL() = %q holds a query string", got)
				}
			})
		}
	})
}

func TestMcStatusEmbedDataDescription(t *testing.T) {
	t.Run("MC-59_an_offline_server_has_a_fixed_description_whatever_else_is_set", func(t *testing.T) {
		cases := []struct {
			name string
			data McStatusEmbedData
		}{
			{"zero_value", McStatusEmbedData{}},
			{"with_details", McStatusEmbedData{Motd: []string{"Hi"}, Players: 3, Max: 20, Version: "1.21"}},
		}
		for _, tc := range cases {
			t.Run("MC-59_"+tc.name, func(t *testing.T) {
				if got := tc.data.Description(); got != "Server offline or unreachable" {
					t.Errorf("Description() = %q", got)
				}
			})
		}
	})

	t.Run("MC-60_an_online_server_shows_the_MOTD_lines_the_players_and_the_version_on_separate_lines", func(t *testing.T) {
		data := McStatusEmbedData{Online: true, Motd: []string{"Line one", "Line two"}, Players: 3, Max: 20, Version: "Paper 1.21.4"}
		if got, want := data.Description(), "Line one\nLine two\nPlayers: 3/20\nVersion: Paper 1.21.4"; got != want {
			t.Errorf("Description() = %q, want %q", got, want)
		}
	})

	t.Run("MC-61_an_online_server_with_no_MOTD_text_has_no_MOTD_line", func(t *testing.T) {
		for name, motd := range map[string][]string{"nil": nil, "one_empty_line": {""}} {
			t.Run("MC-61_"+name, func(t *testing.T) {
				data := McStatusEmbedData{Online: true, Motd: motd, Players: 0, Max: 20, Version: "1.21"}
				if got, want := data.Description(), "Players: 0/20\nVersion: 1.21"; got != want {
					t.Errorf("Description() = %q, want %q", got, want)
				}
			})
		}
	})

	t.Run("MC-62_an_online_server_with_no_version_has_no_version_line", func(t *testing.T) {
		data := McStatusEmbedData{Online: true, Motd: []string{"Hi"}, Players: 0, Max: 0}
		if got, want := data.Description(), "Hi\nPlayers: 0/0"; got != want {
			t.Errorf("Description() = %q, want %q", got, want)
		}
	})
}

func TestMcStatusTruncation(t *testing.T) {
	describe := func(motd []string, version string) string {
		return McStatusEmbedData{Online: true, Motd: motd, Players: 1, Max: 2, Version: version}.Description()
	}

	t.Run("MC-63_the_MOTD_is_cut_after_200_runes_with_an_ellipsis_and_200_runes_pass_unchanged", func(t *testing.T) {
		cases := []struct {
			name  string
			count int
			want  string
		}{
			{"200", 200, strings.Repeat("x", 200)},
			{"201", 201, strings.Repeat("x", 200) + "…"},
		}
		for _, tc := range cases {
			t.Run("MC-63_"+tc.name, func(t *testing.T) {
				if got, want := describe([]string{strings.Repeat("x", tc.count)}, ""), tc.want+"\nPlayers: 1/2"; got != want {
					t.Errorf("Description() = %q, want %q", got, want)
				}
			})
		}
	})

	t.Run("MC-64_the_limit_counts_runes_so_multibyte_text_is_kept_whole_and_never_cut_inside_a_character", func(t *testing.T) {
		cases := []struct {
			name, char string
			count      int
			want       string
		}{
			{"200_two_byte_runes", "é", 200, strings.Repeat("é", 200)},
			{"200_four_byte_runes", "😀", 200, strings.Repeat("😀", 200)},
			{"201_four_byte_runes", "😀", 201, strings.Repeat("😀", 200) + "…"},
		}
		for _, tc := range cases {
			t.Run("MC-64_"+tc.name, func(t *testing.T) {
				got := describe([]string{strings.Repeat(tc.char, tc.count)}, "")
				if want := tc.want + "\nPlayers: 1/2"; got != want {
					t.Errorf("Description() = %q, want %q", got, want)
				}
				if !utf8.ValidString(got) {
					t.Errorf("Description() = %q is not valid UTF-8", got)
				}
			})
		}
	})

	t.Run("MC-65_the_version_is_cut_after_64_runes_with_an_ellipsis_and_64_runes_pass_unchanged", func(t *testing.T) {
		cases := []struct {
			name  string
			count int
			want  string
		}{
			{"64", 64, strings.Repeat("v", 64)},
			{"65", 65, strings.Repeat("v", 64) + "…"},
		}
		for _, tc := range cases {
			t.Run("MC-65_"+tc.name, func(t *testing.T) {
				got := describe(nil, strings.Repeat("v", tc.count))
				if want := "Players: 1/2\nVersion: " + tc.want; got != want {
					t.Errorf("Description() = %q, want %q", got, want)
				}
			})
		}
	})

	t.Run("MC-66_the_MOTD_limit_applies_to_the_lines_joined_with_newlines", func(t *testing.T) {
		got := describe([]string{strings.Repeat("a", 150), strings.Repeat("b", 150)}, "")
		want := strings.Repeat("a", 150) + "\n" + strings.Repeat("b", 49) + "…\nPlayers: 1/2"
		if got != want {
			t.Errorf("Description() = %q, want %q", got, want)
		}
	})
}

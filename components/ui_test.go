package components

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil"
)

func TestHTMXHead(t *testing.T) {
	t.Run("CU-01_the_head_fragment_carries_the_htmx_config_and_both_scripts_with_their_loading_attributes", func(t *testing.T) {
		out := renderString(t, htmxHead())

		meta := regexp.MustCompile(`<meta name="htmx-config" content="([^"]*)"`).FindStringSubmatch(out)
		if meta == nil {
			t.Fatalf("no htmx-config meta in:\n%s", out)
		}
		var config map[string]any
		if err := json.Unmarshal([]byte(html.UnescapeString(meta[1])), &config); err != nil {
			t.Fatalf("htmx-config content %q is not JSON: %v", meta[1], err)
		}
		if config["mode"] != "same-origin" {
			t.Errorf("mode = %v, want same-origin", config["mode"])
		}

		var glue, htmx string
		for _, tag := range regexp.MustCompile(`<script[^>]*>`).FindAllString(out, -1) {
			switch {
			case strings.Contains(tag, ` src="/public/js/htmx-glue.js"`):
				glue = tag
			case strings.Contains(tag, ` src="https://s3.neuralnexus.dev/cdn/htmx/`):
				htmx = tag
			}
		}
		if !strings.Contains(glue, " defer") {
			t.Errorf("glue script tag = %q, want defer", glue)
		}
		for _, want := range []string{` integrity="sha384-`, ` crossorigin="anonymous"`, " defer"} {
			if !strings.Contains(htmx, want) {
				t.Errorf("htmx script tag = %q, want %q", htmx, want)
			}
		}
	})
}

func TestHTMXPage(t *testing.T) {
	t.Run("CU-02_the_page_wraps_the_contents_in_the_layout_and_adds_the_htmx_head", func(t *testing.T) {
		out := renderString(t, htmxPage("Users - NeuralNexus", templ.Raw(`<p id="marker">hello</p>`)))
		if !strings.HasPrefix(strings.ToLower(out), "<!doctype html>") {
			t.Errorf("output starts %q, want a doctype", out[:min(len(out), 40)])
		}
		assertInOrder(t, out, "<title>Users - NeuralNexus</title>")
		assertInOrder(t, out, "<head>", `name="htmx-config"`, `src="/public/js/htmx-glue.js"`, "</head>", "<body", `<main id="main"`, `<p id="marker">hello</p>`, "</main>")
	})

	t.Run("CU-03_a_title_with_markup_is_escaped_and_the_contents_component_is_not", func(t *testing.T) {
		out := renderString(t, htmxPage("<b>A</b> & \"B\"", templ.Raw(`<i>x</i>`)))
		for _, want := range []string{`<title>&lt;b&gt;A&lt;/b&gt; &amp; &#34;B&#34;</title>`, `<i>x</i>`} {
			if !strings.Contains(out, want) {
				t.Errorf("output lacks %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "<b>A") {
			t.Errorf("output holds an unescaped <b>A:\n%s", out)
		}
	})
}

func TestPageShell(t *testing.T) {
	t.Run("CU-04_the_shell_renders_its_children_then_the_error_banner_the_status_line_and_the_loader", func(t *testing.T) {
		out := renderWithChild(t, pageShell("max-w-md", "", "/admin/cards"), `<h2 id="child">c</h2>`)
		if n := strings.Count(out, `id="child"`); n != 1 {
			t.Errorf(`id="child" appears %d times, want 1`, n)
		}
		assertInOrder(t, out, `id="child"`, `id="page-error"`, `id="page-status"`, `data-page-load`)
		if tag := testutil.TagByID(out, "page-error"); !strings.Contains(tag, `role="alert"`) {
			t.Errorf("page-error tag = %q, want role=alert", tag)
		}
		if tag := testutil.TagByID(out, "page-status"); !strings.Contains(tag, `role="status"`) {
			t.Errorf("page-status tag = %q, want role=status", tag)
		}
		if tag := regexp.MustCompile(`<div[^>]*data-page-load[^>]*>`).FindString(out); !strings.Contains(tag, `hx-get="/admin/cards"`) {
			t.Errorf("loader tag = %q, want hx-get=/admin/cards", tag)
		}
		if n := strings.Count(out, `role="status"`); n != 1 {
			t.Errorf(`role="status" appears %d times, want 1`, n)
		}
	})

	t.Run("CU-05_a_non_empty_status_id_adds_one_more_status_line_between_the_page_status_and_the_loader", func(t *testing.T) {
		out := renderWithChild(t, pageShell("max-w-md", "admin-user-status", "/x"), `<h2 id="child">c</h2>`)
		tag := testutil.TagByID(out, "admin-user-status")
		if !strings.HasPrefix(tag, "<p ") || !strings.Contains(tag, `role="status"`) {
			t.Errorf("admin-user-status tag = %q, want a p with role=status", tag)
		}
		assertInOrder(t, out, `id="page-status"`, `id="admin-user-status"`, `data-page-load`)
		if n := strings.Count(out, `role="status"`); n != 2 {
			t.Errorf(`role="status" appears %d times, want 2`, n)
		}
	})
}

func TestPageHeading(t *testing.T) {
	t.Run("CU-06_the_title_is_the_h1_and_the_description_is_a_paragraph", func(t *testing.T) {
		out := renderString(t, pageHeading("Users", "Select an account to edit it"))
		assertInOrder(t, out, "<h1", ">Users</h1>", "<p", ">Select an account to edit it</p>")
	})

	t.Run("CU-07_title_and_description_with_markup_are_escaped", func(t *testing.T) {
		out := renderString(t, pageHeading("<b>T</b>", "a & \"b\""))
		for _, want := range []string{`&lt;b&gt;T&lt;/b&gt;`, `a &amp; &#34;b&#34;`} {
			if !strings.Contains(out, want) {
				t.Errorf("output lacks %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "<b>") {
			t.Errorf("output holds an unescaped <b>:\n%s", out)
		}
	})
}

func TestPageError(t *testing.T) {
	t.Run("CU-08_the_banner_is_an_empty_alert_region_with_the_id_the_error_responses_target", func(t *testing.T) {
		out := renderString(t, pageError())
		tag := testutil.TagByID(out, "page-error")
		if !strings.HasPrefix(tag, "<div ") || !strings.Contains(tag, `role="alert"`) || out != tag+"</div>" {
			t.Errorf("output = %q, want one empty div with id page-error and role=alert", out)
		}
	})
}

func TestPageStatus(t *testing.T) {
	t.Run("CU-09_the_status_line_takes_the_id_it_is_given_and_is_an_empty_status_region", func(t *testing.T) {
		out := renderString(t, pageStatus("admin-role-status"))
		tag := testutil.TagByID(out, "admin-role-status")
		if !strings.HasPrefix(tag, "<p ") || !strings.Contains(tag, `role="status"`) || out != tag+"</p>" {
			t.Errorf("output = %q, want one empty p with id admin-role-status and role=status", out)
		}
	})
}

func TestPageLoad(t *testing.T) {
	t.Run("CU-10_the_loader_fetches_its_path_on_load_and_replaces_itself", func(t *testing.T) {
		out := renderString(t, pageLoad("/account/content"))
		tag := regexp.MustCompile(`<div[^>]*data-page-load[^>]*>`).FindString(out)
		for _, want := range []string{`hx-get="/account/content"`, `hx-trigger="load"`, `hx-swap="outerHTML"`} {
			if !strings.Contains(tag, want) {
				t.Errorf("loader tag = %q, want %q", tag, want)
			}
		}
		if !strings.Contains(out, "Loading…") {
			t.Errorf("output lacks the text Loading…:\n%s", out)
		}
	})

	t.Run("CU-11_a_path_with_quotes_ampersands_and_angle_brackets_stays_inside_the_attribute", func(t *testing.T) {
		out := renderString(t, pageLoad("/a?x=\"1\"&y=<2>"))
		if want := `hx-get="/a?x=&#34;1&#34;&amp;y=&lt;2&gt;"`; !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
		if strings.Contains(out, "<2>") {
			t.Errorf("output holds a raw <2>:\n%s", out)
		}
	})
}

func TestBackLink(t *testing.T) {
	t.Run("CU-12_the_link_has_the_given_href_and_text", func(t *testing.T) {
		out := renderString(t, backLink("/admin", "Back to the admin dashboard"))
		assertInOrder(t, out, `<a href="/admin"`, `>Back to the admin dashboard</a>`)
	})

	t.Run("CU-13_text_with_markup_is_escaped_and_a_script_URL_is_not_rendered_as_a_link_target", func(t *testing.T) {
		out := renderString(t, backLink("javascript:alert(1)", "<b>x</b> & \"y\""))
		if want := `&lt;b&gt;x&lt;/b&gt; &amp; &#34;y&#34;`; !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
		for _, bad := range []string{"<b>", "javascript:"} {
			if strings.Contains(out, bad) {
				t.Errorf("output holds %q:\n%s", bad, out)
			}
		}
	})
}

func TestLinkPair(t *testing.T) {
	t.Run("CU-14_two_links_render_in_the_order_given_each_with_its_own_href_and_text", func(t *testing.T) {
		out := renderString(t, linkPair("/admin", "Back", "/admin/roles", "Roles"))
		assertInOrder(t, out, `<a href="/admin"`, `>Back</a>`, `<a href="/admin/roles"`, `>Roles</a>`)
	})

	t.Run("CU-15_texts_with_markup_are_escaped_and_script_URLs_are_not_rendered", func(t *testing.T) {
		out := renderString(t, linkPair("javascript:a()", "<b>L</b>", "javascript:b()", "R & \"S\""))
		for _, want := range []string{`&lt;b&gt;L&lt;/b&gt;`, `R &amp; &#34;S&#34;`} {
			if !strings.Contains(out, want) {
				t.Errorf("output lacks %q:\n%s", want, out)
			}
		}
		for _, bad := range []string{"<b>", "javascript:"} {
			if strings.Contains(out, bad) {
				t.Errorf("output holds %q:\n%s", bad, out)
			}
		}
	})
}

func TestSeparator(t *testing.T) {
	t.Run("CU-16_the_separator_renders_one_empty_div_that_screen_readers_skip", func(t *testing.T) {
		out := renderString(t, separator())
		if !strings.HasPrefix(out, "<div ") || !strings.Contains(out, `role="none"`) || !strings.HasSuffix(out, "></div>") || strings.Count(out, "<") != 2 {
			t.Errorf("output = %q, want one empty div with role=none", out)
		}
	})
}

func TestEmptyState(t *testing.T) {
	cases := []struct{ name, id, text, shown string }{
		{"plain", "admin-roles-empty", "No roles", "No roles"},
		{"markup", "e-1", "<b>x</b> & \"y\"", `&lt;b&gt;x&lt;/b&gt; &amp; &#34;y&#34;`},
	}
	t.Run("CU-17_the_id_and_text_are_rendered_and_text_with_markup_is_escaped", func(t *testing.T) {
		for _, tc := range cases {
			t.Run("CU-17_"+tc.name, func(t *testing.T) {
				out := renderString(t, emptyState(tc.id, tc.text))
				tag := testutil.TagByID(out, tc.id)
				if !strings.HasPrefix(tag, "<p ") || out != tag+tc.shown+"</p>" {
					t.Errorf("output = %q, want one p with id %q holding %q", out, tc.id, tc.shown)
				}
			})
		}
	})
}

func TestMutedNote(t *testing.T) {
	cases := []struct{ name, id, text, shown string }{
		{"plain", "admin-user-links-empty", "No linked accounts", "No linked accounts"},
		{"markup", "n-1", "<b>x</b> & \"y\"", `&lt;b&gt;x&lt;/b&gt; &amp; &#34;y&#34;`},
	}
	t.Run("CU-18_the_id_and_text_are_rendered_and_text_with_markup_is_escaped", func(t *testing.T) {
		for _, tc := range cases {
			t.Run("CU-18_"+tc.name, func(t *testing.T) {
				out := renderString(t, mutedNote(tc.id, tc.text))
				tag := testutil.TagByID(out, tc.id)
				if !strings.HasPrefix(tag, "<p ") || out != tag+tc.shown+"</p>" {
					t.Errorf("output = %q, want one p with id %q holding %q", out, tc.id, tc.shown)
				}
			})
		}
	})
}

func TestFormField(t *testing.T) {
	t.Run("CU-19_the_field_ties_its_label_to_the_input_id_and_has_no_help_error_or_out_of_band_swap", func(t *testing.T) {
		out := renderWithChild(t, formField("username", "Username", ""), `<input id="username"/>`)
		tag := testutil.TagByID(out, "username-field")
		if !strings.HasPrefix(tag, "<div ") || strings.Contains(tag, "hx-swap-oob") {
			t.Errorf("username-field tag = %q, want a div with no hx-swap-oob", tag)
		}
		assertInOrder(t, out, "<label", `for="username"`, ">Username</label>", `<input id="username"/>`)
		if n := strings.Count(out, `<input id="username"/>`); n != 1 {
			t.Errorf("the child input appears %d times, want 1", n)
		}
		for _, bad := range []string{`id="username-help"`, `id="username-error"`} {
			if strings.Contains(out, bad) {
				t.Errorf("output holds %q:\n%s", bad, out)
			}
		}
	})

	t.Run("CU-20_a_help_text_renders_as_the_help_paragraph_under_the_input", func(t *testing.T) {
		out := renderWithChild(t, formField("role-desc", "Description", "Shown on the role list"), `<input id="role-desc"/>`)
		assertInOrder(t, out, `<input id="role-desc"/>`, `<p id="role-desc-help"`, ">Shown on the role list</p>")
		if strings.Contains(out, `id="role-desc-error"`) {
			t.Errorf("output holds an error id:\n%s", out)
		}
	})
}

func TestFieldWithError(t *testing.T) {
	const child = `<input id="n"/>`

	t.Run("CU-21_the_wrapper_gets_the_id_with_the_field_suffix_and_the_label_points_at_the_input_id", func(t *testing.T) {
		out := renderWithChild(t, fieldWithError("admin-role-name", "Name", "", "", false), `<input id="admin-role-name"/>`)
		tag := testutil.TagByID(out, "admin-role-name-field")
		if !strings.HasPrefix(tag, "<div ") || strings.Contains(tag, "hx-swap-oob") {
			t.Errorf("admin-role-name-field tag = %q, want a div with no hx-swap-oob", tag)
		}
		assertInOrder(t, out, "<label", `for="admin-role-name"`, ">Name</label>", `<input id="admin-role-name"/>`)
		for _, bad := range []string{`id="admin-role-name-help"`, `id="admin-role-name-error"`} {
			if strings.Contains(out, bad) {
				t.Errorf("output holds %q:\n%s", bad, out)
			}
		}
	})

	t.Run("CU-22_a_help_text_renders_as_a_paragraph_whose_id_is_the_field_id_plus_help", func(t *testing.T) {
		out := renderWithChild(t, fieldWithError("n", "Name", "Lowercase letters only", "", false), child)
		assertInOrder(t, out, child, `<p id="n-help"`, ">Lowercase letters only</p>")
		if strings.Contains(out, `id="n-error"`) {
			t.Errorf("output holds an error id:\n%s", out)
		}
	})

	t.Run("CU-23_an_error_text_renders_as_a_paragraph_whose_id_is_the_field_id_plus_error", func(t *testing.T) {
		out := renderWithChild(t, fieldWithError("n", "Name", "", "Name is taken", false), child)
		assertInOrder(t, out, child, `<p id="n-error"`, ">Name is taken</p>")
		if strings.Contains(out, `id="n-help"`) {
			t.Errorf("output holds a help id:\n%s", out)
		}
	})

	t.Run("CU-24_help_and_error_render_together_help_first", func(t *testing.T) {
		out := renderWithChild(t, fieldWithError("n", "Name", "Lowercase letters only", "Name is taken", false), child)
		assertInOrder(t, out, child, `id="n-help"`, `id="n-error"`)
		for _, id := range []string{`id="n-help"`, `id="n-error"`} {
			if n := strings.Count(out, id); n != 1 {
				t.Errorf("%s appears %d times, want 1", id, n)
			}
		}
	})

	t.Run("CU-25_the_out_of_band_flag_adds_the_swap_attribute_to_the_wrapper_and_nothing_else_changes", func(t *testing.T) {
		plain := renderWithChild(t, fieldWithError("n", "Name", "", "", false), child)
		for _, oob := range []bool{true, false} {
			name := "false"
			if oob {
				name = "true"
			}
			t.Run("CU-25_"+name, func(t *testing.T) {
				out := renderWithChild(t, fieldWithError("n", "Name", "", "", oob), child)
				if !oob {
					if strings.Contains(out, "hx-swap-oob") {
						t.Errorf("output holds hx-swap-oob:\n%s", out)
					}
					return
				}
				if tag := testutil.TagByID(out, "n-field"); !strings.Contains(tag, `hx-swap-oob="true"`) {
					t.Errorf("n-field tag = %q, want hx-swap-oob=true", tag)
				}
				if got := strings.Replace(out, ` hx-swap-oob="true"`, "", 1); got != plain {
					t.Errorf("output without the attribute = %q, want %q", got, plain)
				}
			})
		}
	})

	t.Run("CU-26_label_help_and_error_text_with_markup_are_escaped", func(t *testing.T) {
		out := renderWithChild(t, fieldWithError("n", "<b>L</b>", "<i>h</i> & \"x\"", "<u>e</u>", false), child)
		for _, want := range []string{`&lt;b&gt;L&lt;/b&gt;`, `&lt;i&gt;h&lt;/i&gt; &amp; &#34;x&#34;`, `&lt;u&gt;e&lt;/u&gt;`} {
			if !strings.Contains(out, want) {
				t.Errorf("output lacks %q:\n%s", want, out)
			}
		}
		for _, bad := range []string{"<b>", "<i>", "<u>"} {
			if strings.Contains(out, bad) {
				t.Errorf("output holds %q:\n%s", bad, out)
			}
		}
	})
}

func TestFieldDescription(t *testing.T) {
	t.Run("CU-27_the_ids_named_depend_on_which_of_help_and_error_are_non_empty_error_first", func(t *testing.T) {
		cases := []struct{ name, help, errText, want string }{
			{"neither", "", "", ""},
			{"help_only", "h", "", "x-help"},
			{"error_only", "", "e", "x-error"},
			{"both", "h", "e", "x-error x-help"},
		}
		for _, tc := range cases {
			t.Run("CU-27_"+tc.name, func(t *testing.T) {
				if got := fieldDescription("x", tc.help, tc.errText); got != tc.want {
					t.Errorf("fieldDescription(x, %q, %q) = %q, want %q", tc.help, tc.errText, got, tc.want)
				}
			})
		}
	})

	t.Run("CU-28_every_id_the_description_names_is_the_id_of_an_element_the_field_renders_and_the_input_is_marked_invalid", func(t *testing.T) {
		input := renderString(t, textInput("x", "name", "v", "", true, fieldDescription("x", "hint", "bad"), true))
		out := renderWithChild(t, fieldWithError("x", "Name", "hint", "bad", false), input)

		tag := testutil.TagByID(out, "x")
		described := regexp.MustCompile(`aria-describedby="([^"]*)"`).FindStringSubmatch(tag)
		if described == nil || described[1] != "x-error x-help" {
			t.Fatalf("input tag = %q, want aria-describedby=x-error x-help", tag)
		}
		for token, text := range map[string]string{"x-error": "bad", "x-help": "hint"} {
			if n := strings.Count(out, ` id="`+token+`"`); n != 1 {
				t.Errorf("id %q appears %d times, want 1", token, n)
			}
			if got := testutil.TagByID(out, token); !strings.HasPrefix(got, "<p ") || !strings.Contains(out, got+text+"</p>") {
				t.Errorf("element %q = %q, want a p holding %q", token, got, text)
			}
		}
		if !strings.Contains(out, `<label class="`) || !strings.Contains(out, ` for="x"`) {
			t.Errorf("the label does not point at x:\n%s", out)
		}
		if !strings.Contains(tag, `aria-invalid="true"`) {
			t.Errorf("input tag = %q, want aria-invalid=true", tag)
		}
	})
}

func TestTextInput(t *testing.T) {
	t.Run("CU-29_with_only_the_required_arguments_the_input_has_its_id_name_and_fixed_attributes_and_none_of_the_optional_ones", func(t *testing.T) {
		tag := testutil.TagByID(renderString(t, textInput("u", "username", "", "", false, "", false)), "u")
		if !strings.HasPrefix(tag, "<input ") {
			t.Fatalf("tag = %q, want an input", tag)
		}
		for _, want := range []string{`name="username"`, `type="text"`, `autocomplete="off"`, `autocapitalize="off"`, `spellcheck="false"`} {
			if !strings.Contains(tag, want) {
				t.Errorf("tag = %q, want %q", tag, want)
			}
		}
		for _, bad := range []string{" required", " value=", " placeholder=", " aria-describedby=", " aria-invalid=", " autofocus"} {
			if strings.Contains(tag, bad) {
				t.Errorf("tag = %q, holds %q", tag, bad)
			}
		}
	})

	t.Run("CU-30_the_required_flag_adds_the_required_attribute", func(t *testing.T) {
		for _, required := range []bool{true, false} {
			name := "false"
			if required {
				name = "true"
			}
			t.Run("CU-30_"+name, func(t *testing.T) {
				tag := testutil.TagByID(renderString(t, textInput("u", "n", "", "", required, "", false)), "u")
				if got := strings.Contains(tag, " required"); got != required {
					t.Errorf("tag = %q, required present = %t, want %t", tag, got, required)
				}
			})
		}
	})

	t.Run("CU-31_a_non_empty_value_and_placeholder_render_as_attributes_and_empty_ones_are_left_out", func(t *testing.T) {
		cases := []struct {
			name, value, placeholder string
			want, lacks              []string
		}{
			{"both_set", "alice", "moderator", []string{`value="alice"`, `placeholder="moderator"`}, nil},
			{"both_empty", "", "", nil, []string{" value=", " placeholder="}},
		}
		for _, tc := range cases {
			t.Run("CU-31_"+tc.name, func(t *testing.T) {
				tag := testutil.TagByID(renderString(t, textInput("u", "n", tc.value, tc.placeholder, false, "", false)), "u")
				for _, want := range tc.want {
					if !strings.Contains(tag, want) {
						t.Errorf("tag = %q, want %q", tag, want)
					}
				}
				for _, bad := range tc.lacks {
					if strings.Contains(tag, bad) {
						t.Errorf("tag = %q, holds %q", tag, bad)
					}
				}
			})
		}
	})

	t.Run("CU-32_value_placeholder_id_name_and_description_with_quotes_and_markup_stay_inside_their_attributes", func(t *testing.T) {
		out := renderString(t, textInput("a\"b", "n\"<", "v\" onfocus=\"x", "p\" onclick=\"y", false, "d\" onblur=\"z", true))
		tag := testutil.TagByID(out, `a&#34;b`)
		if tag == "" {
			t.Fatalf("no tag with the escaped id in:\n%s", out)
		}
		for _, want := range []string{`value="v&#34; onfocus=&#34;x"`, `placeholder="p&#34; onclick=&#34;y"`, `aria-describedby="d&#34; onblur=&#34;z"`} {
			if !strings.Contains(tag, want) {
				t.Errorf("tag = %q, want %q", tag, want)
			}
		}
		for _, bad := range []string{` onfocus="`, ` onclick="`, ` onblur="`} {
			if strings.Contains(tag, bad) {
				t.Errorf("tag = %q, holds the attribute %q", tag, bad)
			}
		}
	})

	t.Run("CU-33_a_non_empty_describedBy_renders_as_aria_describedby_and_an_empty_one_renders_nothing", func(t *testing.T) {
		cases := []struct{ name, describedBy string }{
			{"set", "x-error x-help"},
			{"empty", ""},
		}
		for _, tc := range cases {
			t.Run("CU-33_"+tc.name, func(t *testing.T) {
				tag := testutil.TagByID(renderString(t, textInput("u", "n", "", "", false, tc.describedBy, false)), "u")
				if tc.describedBy != "" {
					if want := `aria-describedby="x-error x-help"`; !strings.Contains(tag, want) {
						t.Errorf("tag = %q, want %q", tag, want)
					}
				} else if strings.Contains(tag, "aria-describedby") {
					t.Errorf("tag = %q, holds aria-describedby", tag)
				}
			})
		}
	})

	t.Run("CU-34_the_invalid_flag_adds_aria_invalid_and_autofocus", func(t *testing.T) {
		for _, invalid := range []bool{true, false} {
			name := "false"
			if invalid {
				name = "true"
			}
			t.Run("CU-34_"+name, func(t *testing.T) {
				tag := testutil.TagByID(renderString(t, textInput("u", "n", "", "", false, "", invalid)), "u")
				if got := strings.Contains(tag, ` aria-invalid="true"`); got != invalid {
					t.Errorf("tag = %q, aria-invalid present = %t, want %t", tag, got, invalid)
				}
				if got := strings.Contains(tag, " autofocus"); got != invalid {
					t.Errorf("tag = %q, autofocus present = %t, want %t", tag, got, invalid)
				}
			})
		}
	})
}

func TestProseInput(t *testing.T) {
	t.Run("CU-35_the_input_has_its_id_name_and_no_value_attribute_when_the_value_is_empty", func(t *testing.T) {
		cases := []struct {
			name, value string
			hasValue    bool
		}{
			{"with_a_value", "A role", true},
			{"empty_value", "", false},
		}
		for _, tc := range cases {
			t.Run("CU-35_"+tc.name, func(t *testing.T) {
				tag := testutil.TagByID(renderString(t, proseInput("d", "description", tc.value)), "d")
				for _, want := range []string{`name="description"`, `type="text"`, `autocomplete="off"`} {
					if !strings.Contains(tag, want) {
						t.Errorf("tag = %q, want %q", tag, want)
					}
				}
				if got := strings.Contains(tag, ` value="A role"`); got != tc.hasValue {
					t.Errorf("tag = %q, value present = %t, want %t", tag, got, tc.hasValue)
				}
				if !tc.hasValue && strings.Contains(tag, " value=") {
					t.Errorf("tag = %q, holds a value attribute", tag)
				}
				for _, bad := range []string{" required", " aria-describedby=", " aria-invalid="} {
					if strings.Contains(tag, bad) {
						t.Errorf("tag = %q, holds %q", tag, bad)
					}
				}
			})
		}
	})

	t.Run("CU-36_an_id_name_and_value_with_quotes_and_markup_stay_inside_their_attributes", func(t *testing.T) {
		out := renderString(t, proseInput("a\"b", "n\"<", "v\" onfocus=\"x"))
		tag := testutil.TagByID(out, `a&#34;b`)
		if tag == "" {
			t.Fatalf("no tag with the escaped id in:\n%s", out)
		}
		for _, want := range []string{`name="n&#34;&lt;"`, `value="v&#34; onfocus=&#34;x"`} {
			if !strings.Contains(tag, want) {
				t.Errorf("tag = %q, want %q", tag, want)
			}
		}
		if strings.Contains(tag, ` onfocus="`) {
			t.Errorf("tag = %q, holds an onfocus attribute", tag)
		}
	})
}

func TestToggle(t *testing.T) {
	t.Run("CU-37_the_toggle_puts_its_child_inside_one_label", func(t *testing.T) {
		out := renderWithChild(t, toggle(), `<input id="t" type="checkbox"/>`)
		assertInOrder(t, out, "<label", `<input id="t" type="checkbox"/>`, "</label>")
	})
}

func TestEntityHeader(t *testing.T) {
	check := func(t *testing.T, prefix, title, subtitle string, oob bool) {
		t.Helper()
		out := renderString(t, entityHeader(prefix, title, subtitle, oob))
		header := testutil.TagByID(out, prefix+"-header")
		if !strings.HasPrefix(header, "<div ") || strings.Contains(header, "hx-swap-oob") != oob {
			t.Errorf("header tag = %q, want a div with hx-swap-oob present = %t", header, oob)
		}
		if h1 := testutil.TagByID(out, prefix+"-title"); !strings.HasPrefix(h1, "<h1 ") || !strings.Contains(out, h1+title+"</h1>") {
			t.Errorf("title tag = %q, want an h1 holding %q in:\n%s", h1, title, out)
		}
		if p := testutil.TagByID(out, prefix+"-id"); !strings.HasPrefix(p, "<p ") || !strings.Contains(out, p+subtitle+"</p>") {
			t.Errorf("subtitle tag = %q, want a p holding %q in:\n%s", p, subtitle, out)
		}
	}

	t.Run("CU-38_the_header_title_and_subtitle_take_ids_from_the_prefix", func(t *testing.T) {
		check(t, "admin-user", "alice", "u-123", false)
	})

	t.Run("CU-39_the_out_of_band_flag_adds_the_swap_attribute_to_the_header", func(t *testing.T) {
		check(t, "admin-role", "mods", "r-1", true)
	})

	t.Run("CU-40_title_and_subtitle_with_markup_are_escaped", func(t *testing.T) {
		out := renderString(t, entityHeader("p", "<b>T</b>", "a & \"b\"", false))
		for _, want := range []string{`&lt;b&gt;T&lt;/b&gt;`, `a &amp; &#34;b&#34;`} {
			if !strings.Contains(out, want) {
				t.Errorf("output lacks %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "<b>") {
			t.Errorf("output holds an unescaped <b>:\n%s", out)
		}
	})
}

func TestErrorClear(t *testing.T) {
	t.Run("CU-41_the_clear_fragment_targets_the_banner_id_with_an_inner_HTML_out_of_band_swap_and_no_content", func(t *testing.T) {
		out := renderString(t, ErrorClear())
		tag := testutil.TagByID(out, "page-error")
		if !strings.HasPrefix(tag, "<div ") || !strings.Contains(tag, `hx-swap-oob="innerHTML"`) || out != tag+"</div>" {
			t.Errorf("output = %q, want one empty div with id page-error and hx-swap-oob=innerHTML", out)
		}
	})
}

func TestStatusLine(t *testing.T) {
	t.Run("CU-42_the_status_line_takes_its_id_and_message_and_swaps_the_inner_HTML_out_of_band", func(t *testing.T) {
		out := renderString(t, StatusLine("admin-user-status", "Saved"))
		tag := testutil.TagByID(out, "admin-user-status")
		if !strings.HasPrefix(tag, "<p ") || !strings.Contains(tag, `hx-swap-oob="innerHTML"`) || out != tag+"Saved</p>" {
			t.Errorf("output = %q, want one p with id admin-user-status, hx-swap-oob=innerHTML and the text Saved", out)
		}
		if strings.Contains(tag, "role=") {
			t.Errorf("tag = %q, holds a role", tag)
		}
	})

	t.Run("CU-43_an_empty_message_still_renders_the_element_so_that_the_swap_clears_the_line", func(t *testing.T) {
		out := renderString(t, StatusLine("admin-user-status", ""))
		tag := testutil.TagByID(out, "admin-user-status")
		if !strings.HasPrefix(tag, "<p ") || !strings.Contains(tag, `hx-swap-oob="innerHTML"`) || out != tag+"</p>" {
			t.Errorf("output = %q, want one empty p with id admin-user-status and hx-swap-oob=innerHTML", out)
		}
	})

	t.Run("CU-44_message_and_id_with_markup_and_quotes_are_escaped", func(t *testing.T) {
		out := renderString(t, StatusLine("s\"<", "<b>x</b> & \"y\""))
		for _, want := range []string{`id="s&#34;&lt;"`, `&lt;b&gt;x&lt;/b&gt; &amp; &#34;y&#34;`} {
			if !strings.Contains(out, want) {
				t.Errorf("output lacks %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "<b>") {
			t.Errorf("output holds an unescaped <b>:\n%s", out)
		}
	})
}

func TestErrorText(t *testing.T) {
	t.Run("CU-45_the_component_writes_the_message_and_no_wrapper_element", func(t *testing.T) {
		if got := renderString(t, ErrorText("Failed to load permissions")); got != "Failed to load permissions" {
			t.Errorf("output = %q", got)
		}
	})

	t.Run("CU-46_a_message_with_markup_is_escaped", func(t *testing.T) {
		got := renderString(t, ErrorText("<script>x</script> & \"y\""))
		if want := `&lt;script&gt;x&lt;/script&gt; &amp; &#34;y&#34;`; got != want {
			t.Errorf("output = %q, want %q", got, want)
		}
	})
}

func TestRowGone(t *testing.T) {
	t.Run("CU-47_the_fragment_is_an_empty_div_that_deletes_the_row_with_the_given_id", func(t *testing.T) {
		got := renderString(t, RowGone("permission-7"))
		if want := `<div hx-swap-oob="delete:#permission-7"></div>`; got != want {
			t.Errorf("output = %q, want %q", got, want)
		}
	})

	t.Run("CU-48_an_id_with_quotes_and_markup_stays_inside_the_attribute", func(t *testing.T) {
		out := renderString(t, RowGone("a\"><script>"))
		if want := `hx-swap-oob="delete:#a&#34;&gt;&lt;script&gt;"`; !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
		if strings.Contains(out, "<script>") {
			t.Errorf("output holds a raw <script>:\n%s", out)
		}
	})
}

func TestHTMXHeadScriptOrder(t *testing.T) {
	t.Run("CU-49_the_glue_script_comes_before_the_htmx_script", func(t *testing.T) {
		out := renderString(t, htmxHead())
		glue, htmx := strings.Index(out, `src="/public/js/htmx-glue.js"`), strings.Index(out, "htmx.min.js")
		if glue < 0 || htmx < 0 || glue > htmx {
			t.Errorf("htmx-glue.js at %d, htmx.min.js at %d, want the glue first so it hears the first request", glue, htmx)
		}
	})
}

func TestPageErrorColours(t *testing.T) {
	t.Run("CU-50_the_banner_uses_the_colours_that_pass_contrast_in_both_themes_and_collapses_when_empty", func(t *testing.T) {
		out := renderString(t, pageError())
		for _, want := range []string{"border-red-700 bg-red-700/10", "text-red-700 dark:border-red-400 dark:bg-red-400/10 dark:text-red-400", "empty:m-0 empty:border-0 empty:p-0"} {
			if !strings.Contains(out, want) {
				t.Errorf("banner lacks %q:\n%s", want, out)
			}
		}
		for _, unwanted := range []string{"empty:hidden", "text-destructive"} {
			if strings.Contains(out, unwanted) {
				t.Errorf("banner holds %q:\n%s", unwanted, out)
			}
		}
	})
}

func TestToggleSwitchClasses(t *testing.T) {
	t.Run("CU-51_the_toggle_is_a_switch_track_with_a_visible_focus_ring", func(t *testing.T) {
		out := renderWithChild(t, toggle(), `<input id="t" type="checkbox"/>`)
		for _, want := range []string{"h-6 w-11", "bg-control", "has-[:focus-visible]:ring-2", "has-[:focus-visible]:ring-ring", "has-[:checked]:bg-primary"} {
			if !strings.Contains(out, want) {
				t.Errorf("toggle lacks %q:\n%s", want, out)
			}
		}
	})
}

func TestSelfReplacingRoots(t *testing.T) {
	t.Run("CU-52_the_roots_that_replace_themselves_are_busy_regions_and_name_no_status_line", func(t *testing.T) {
		roots := map[string]string{
			"account":     renderString(t, AccountContent(AccountData{})),
			"permissions": renderString(t, AdminPermissionsContent(AdminPermissionsData{})),
			"bee":         renderString(t, BeeSuggestions(BeeSuggestionsData{})),
		}
		for name, out := range roots {
			if !strings.Contains(out, "data-busy-region") {
				t.Errorf("%s: no data-busy-region in:\n%s", name, out)
			}
			if strings.Contains(out, "data-status") {
				t.Errorf("%s: holds data-status in:\n%s", name, out)
			}
		}
	})
}

package components

import (
	"html"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil"
)

func assertAttr(t testing.TB, page, id, name, want string) {
	t.Helper()
	tag := testutil.TagByID(page, id)
	if tag == "" {
		t.Errorf("no element with id %q in:\n%s", id, page)
		return
	}
	if got, ok := tagAttr(tag, name); !ok || got != want {
		t.Errorf("%s of %q = %q (present %t), want %q; tag %s", name, id, got, ok, want, tag)
	}
}

func assertFlag(t testing.TB, page, id, name string, want bool) {
	t.Helper()
	tag := testutil.TagByID(page, id)
	if tag == "" {
		t.Errorf("no element with id %q in:\n%s", id, page)
		return
	}
	if _, ok := tagAttr(tag, name); ok != want {
		t.Errorf("%s on %q present = %t, want %t; tag %s", name, id, ok, want, tag)
	}
}

func assertText(t testing.TB, page, id, want string) {
	t.Helper()
	inner, ok := innerHTML(page, id)
	if !ok {
		t.Errorf("no element with id %q in:\n%s", id, page)
		return
	}
	if got := strings.TrimSpace(inner); got != want {
		t.Errorf("text of %q = %q, want %q", id, got, want)
	}
}

func assertOnce(t testing.TB, page, id string) {
	t.Helper()
	if n := strings.Count(page, ` id="`+id+`"`); n != 1 {
		t.Errorf("id %q occurs %d times, want 1", id, n)
	}
}

func assertElement(t testing.TB, page, id, element string) {
	t.Helper()
	if tag := testutil.TagByID(page, id); !strings.HasPrefix(tag, "<"+element+" ") && !strings.HasPrefix(tag, "<"+element+">") {
		t.Errorf("element %q = %q, want a %s", id, tag, element)
	}
}

func assertHasClass(t testing.TB, page, id, class string, want bool) {
	t.Helper()
	value, _ := tagAttr(testutil.TagByID(page, id), "class")
	found := false
	for _, field := range strings.Fields(value) {
		found = found || field == class
	}
	if found != want {
		t.Errorf("class %q on %q present = %t, want %t; classes %q", class, id, found, want, value)
	}
}

func assertContains(t testing.TB, page string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(page, want) {
			t.Errorf("the output lacks %q:\n%s", want, page)
		}
	}
}

func assertLacks(t testing.TB, page string, unwanted ...string) {
	t.Helper()
	for _, bad := range unwanted {
		if strings.Contains(page, bad) {
			t.Errorf("the output holds %q:\n%s", bad, page)
		}
	}
}

func lastScript(t testing.TB, page string) string {
	t.Helper()
	at := strings.LastIndex(page, "<script>")
	if at < 0 {
		t.Fatalf("no inline script in:\n%s", page)
	}
	return page[at:]
}

func TestSteamOpenIDLoginURL(t *testing.T) {
	t.Run("CP-01_a_set_STEAM_OPENID_LOGIN_URL_is_returned_as_given", func(t *testing.T) {
		t.Setenv("STEAM_OPENID_LOGIN_URL", "https://steam.test/openid/login")
		if got := steamOpenIDLoginURL(); got != "https://steam.test/openid/login" {
			t.Errorf("steamOpenIDLoginURL() = %q", got)
		}
	})

	t.Run("CP-02_an_unset_or_empty_variable_falls_back_to_the_Steam_login_endpoint", func(t *testing.T) {
		for _, state := range []string{"unset", "empty"} {
			t.Run("CP-02_"+state, func(t *testing.T) {
				t.Setenv("STEAM_OPENID_LOGIN_URL", "")
				if state == "unset" {
					os.Unsetenv("STEAM_OPENID_LOGIN_URL")
				}
				if got := steamOpenIDLoginURL(); got != "https://steamcommunity.com/openid/login" {
					t.Errorf("steamOpenIDLoginURL() = %q", got)
				}
			})
		}
	})
}

func TestWrapContents(t *testing.T) {
	t.Run("CP-03_the_default_layout_uses_the_title_NeuralNexus_loads_no_htmx_and_shows_the_contents_inside_main", func(t *testing.T) {
		out := renderString(t, WrapContents(templ.Raw(`<p id="probe">x</p>`)))
		assertContains(t, out, "<title>NeuralNexus</title>")
		assertInOrder(t, out, "<main", `id="probe"`, "</main>")
		assertLacks(t, out, "htmx.min.js", "htmx-glue.js")
	})
}

func TestWrapContentsWithHead(t *testing.T) {
	probe := templ.Raw(`<p id="probe">x</p>`)

	t.Run("CP-04_the_document_declares_its_type_language_encoding_viewport_and_shared_assets", func(t *testing.T) {
		out := renderString(t, WrapContentsWithHead("T", nil, probe))
		if !strings.HasPrefix(out, "<!doctype html>") {
			t.Errorf("output starts %q, want <!doctype html>", out[:min(len(out), 30)])
		}
		assertContains(t, out, `<html lang="en">`, `<meta charset="UTF-8">`, `<meta name="viewport" content="width=device-width, initial-scale=1.0">`)
		for _, asset := range []string{"/public/css/styles.css", "/public/js/scripts.js", "/public/js/theming.js"} {
			if n := strings.Count(out, asset); n != 1 {
				t.Errorf("%s is referenced %d times, want 1", asset, n)
			}
			assertInOrder(t, out, asset, "</head>")
		}
	})

	t.Run("CP-05_the_title_text_is_escaped", func(t *testing.T) {
		out := renderString(t, WrapContentsWithHead("A<b>&\"x\"", nil, probe))
		assertContains(t, out, `<title>A&lt;b&gt;&amp;&#34;x&#34;</title>`)
		if title := out[strings.Index(out, "<title>"):strings.Index(out, "</title>")]; strings.Contains(title, "<b>") {
			t.Errorf("the title holds <b>: %q", title)
		}
	})

	t.Run("CP-06_the_skip_link_comes_first_and_targets_the_single_main_landmark", func(t *testing.T) {
		out := renderString(t, WrapContentsWithHead("T", nil, probe))
		body := out[strings.Index(out, "<body"):]
		link := regexp.MustCompile(`<a[^>]*>`).FindString(body)
		if href, _ := tagAttr(link, "href"); href != "#main" {
			t.Errorf("first link = %q, want href #main", link)
		}
		if !strings.HasPrefix(body[strings.Index(body, link)+len(link):], "Skip to main content</a>") {
			t.Errorf("the first link does not read Skip to main content: %q", link)
		}
		if at := strings.Index(body, link); at > strings.Index(body, "<header") || at > strings.Index(body, "<main") {
			t.Errorf("the skip link does not come before the header and main")
		}
		assertAttr(t, out, "main", "tabindex", "-1")
		if n := strings.Count(out, "<main"); n != 1 {
			t.Errorf("<main appears %d times, want 1", n)
		}
	})

	t.Run("CP-122_the_skip_link_shows_only_on_focus_and_the_main_landmark_takes_focus_without_an_outline", func(t *testing.T) {
		out := renderString(t, WrapContentsWithHead("T", nil, probe))
		link := regexp.MustCompile(`<a[^>]*href="#main"[^>]*>`).FindString(out)
		for _, want := range []string{"sr-only", "focus:not-sr-only", "focus:absolute", "focus:ring-2"} {
			if !strings.Contains(link, want) {
				t.Errorf("skip link %q lacks %q", link, want)
			}
		}
		main := regexp.MustCompile(`<main[^>]*>`).FindString(out)
		for _, want := range []string{"focus:outline-none", "relative isolate px-6 pt-14 lg:px-8"} {
			if !strings.Contains(main, want) {
				t.Errorf("main tag %q lacks %q", main, want)
			}
		}
	})

	t.Run("CP-07_the_header_sits_before_main_and_the_contents_sit_inside_main", func(t *testing.T) {
		out := renderString(t, WrapContentsWithHead("T", nil, probe))
		assertInOrder(t, out, "<header", "<main", `id="probe"`, "</main>", "</body>")
	})

	t.Run("CP-08_the_API_base_URL_is_published_in_a_hidden_element_for_the_scripts", func(t *testing.T) {
		out := renderString(t, WrapContentsWithHead("T", nil, probe))
		assertFlag(t, out, "api-base-url", "hidden", true)
		assertText(t, out, "api-base-url", "http://api.test")
	})

	t.Run("CP-09_the_Steam_login_URL_is_published_in_a_hidden_element_and_is_escaped", func(t *testing.T) {
		previous := STEAM_OPENID_LOGIN_URL
		t.Cleanup(func() { STEAM_OPENID_LOGIN_URL = previous })
		STEAM_OPENID_LOGIN_URL = "https://steam.test/openid?a=1&b=2"
		out := renderString(t, WrapContentsWithHead("T", nil, probe))
		assertFlag(t, out, "steam-openid-login-url", "hidden", true)
		assertText(t, out, "steam-openid-login-url", "https://steam.test/openid?a=1&amp;b=2")
	})

	t.Run("CP-10_a_non_nil_head_component_renders_inside_the_head_and_nowhere_else", func(t *testing.T) {
		out := renderString(t, WrapContentsWithHead("T", templ.Raw(`<meta name="probe" content="1">`), probe))
		if n := strings.Count(out, `name="probe"`); n != 1 {
			t.Errorf("the probe meta appears %d times, want 1", n)
		}
		assertInOrder(t, out, "<head>", `name="probe"`, "</head>", "<body")
	})

	t.Run("CP-11_a_nil_head_renders_no_extra_head_content", func(t *testing.T) {
		out := renderString(t, WrapContentsWithHead("T", nil, probe))
		assertContains(t, out, `<script src="/public/js/theming.js"></script></head>`)
	})
}

func TestHomePage(t *testing.T) {
	t.Run("CP-12_the_home_page_is_the_home_content_in_the_default_layout", func(t *testing.T) {
		out := renderString(t, HomePage())
		assertContains(t, out, "<title>NeuralNexus</title>")
		assertInOrder(t, out, "<main", `id="auth-error"`, "</main>")
	})
}

func TestHome(t *testing.T) {
	t.Run("CP-13_the_home_content_shows_the_error_banner_the_site_heading_and_the_script_that_reveals_auth_errors", func(t *testing.T) {
		out := renderString(t, Home())
		assertOnce(t, out, "auth-error")
		if !regexp.MustCompile(`<h1[^>]*>\s*NeuralNexus\s*</h1>`).MatchString(out) {
			t.Errorf("no h1 NeuralNexus in:\n%s", out)
		}
		if script := lastScript(t, out); !strings.Contains(script, "showAuthErrorFromQuery();") || strings.LastIndex(out, "<script>") < strings.Index(out, `id="auth-error"`) {
			t.Errorf("the script after the banner = %q", script)
		}
	})
}

func TestErrorBanner(t *testing.T) {
	t.Run("CP-14_the_banner_is_an_empty_element_hidden_until_a_script_reveals_it", func(t *testing.T) {
		out := renderString(t, ErrorBanner())
		assertFlag(t, out, "auth-error", "hidden", true)
		assertText(t, out, "auth-error", "")
		assertOnce(t, out, "auth-error")
	})
}

func TestThemeToggle(t *testing.T) {
	t.Run("CP-15_the_toggle_button_calls_the_theme_function_and_has_both_mode_icons_that_theming_js_switches", func(t *testing.T) {
		out := renderString(t, ThemeToggle())
		button := regexp.MustCompile(`<button[^>]*>`).FindString(out)
		if got, _ := tagAttr(button, "onclick"); got != "toggleTheme()" {
			t.Errorf("button = %q, want onclick toggleTheme()", button)
		}
		for _, id := range []string{"light-mode", "dark-mode"} {
			assertOnce(t, out, id)
			assertElement(t, out, id, "span")
			assertFlag(t, out, id, "hidden", false)
		}
	})
}

func TestHeader(t *testing.T) {
	t.Run("CP-16_the_main_navigation_is_a_labelled_landmark_with_the_four_site_links_in_order", func(t *testing.T) {
		out := renderString(t, Header())
		nav := regexp.MustCompile(`<nav[^>]*>`).FindString(out)
		if got, _ := tagAttr(nav, "aria-label"); got != "Main" {
			t.Errorf("nav = %q, want aria-label Main", nav)
		}
		assertInOrder(t, out, "<header", nav, "</nav>", "</header>")
		var links []string
		for _, link := range regexp.MustCompile(`<a href="([^"]*)">([^<]*)</a>`).FindAllStringSubmatch(out[strings.Index(out, nav):strings.Index(out, "</nav>")], -1) {
			links = append(links, link[1]+" "+link[2])
		}
		if want := []string{"/ NeuralNexus", "/about About", "/contact Contact", "/projects Projects"}; strings.Join(links, "|") != strings.Join(want, "|") {
			t.Errorf("links = %q, want %q", links, want)
		}
	})

	t.Run("CP-17_the_header_holds_the_theme_toggle_and_the_sign_in_controls", func(t *testing.T) {
		out := renderString(t, Header())
		for _, id := range []string{"light-mode", "header-account", "header-logout-btn", "header-login-btn"} {
			assertOnce(t, out, id)
		}
		assertInOrder(t, out, "<header", "<nav", "</nav>", `id="light-mode"`, `id="header-account"`, `id="header-logout-btn"`, `id="header-login-btn"`, "</header>")
	})
}

func TestProjectCardBody(t *testing.T) {
	t.Run("CP-18_the_card_body_shows_the_icon_hidden_from_assistive_tech_then_the_title_and_description_with_the_children_beside_the_icon", func(t *testing.T) {
		out := renderWithChild(t, projectCardBody("🐝", "T", "D"), `<span id="kid">k</span>`)
		icon := regexp.MustCompile(`<span[^>]*aria-hidden[^>]*>`).FindString(out)
		if got, _ := tagAttr(icon, "aria-hidden"); got != "true" {
			t.Errorf("icon span = %q, want aria-hidden true", icon)
		}
		assertInOrder(t, out, icon+"🐝</span>", `id="kid"`, "<h2", ">T</h2>", ">D</p>")
	})
}

func TestProjectCard(t *testing.T) {
	t.Run("CP-19_the_card_is_one_link_to_the_given_address_that_holds_the_body_and_a_Live_badge", func(t *testing.T) {
		out := renderString(t, projectCard("/project/x", "I", "T", "D"))
		if n := strings.Count(out, "<a "); n != 1 {
			t.Errorf("the card holds %d links, want 1", n)
		}
		if href, _ := tagAttr(regexp.MustCompile(`<a[^>]*>`).FindString(out), "href"); href != "/project/x" {
			t.Errorf("href = %q, want /project/x", href)
		}
		assertContains(t, out, ">I</span>", ">T</h2>", ">D</p>", ">Live</span>")
	})
}

func TestProjectSection(t *testing.T) {
	t.Run("CP-20_the_section_shows_its_heading_and_description_then_its_children", func(t *testing.T) {
		out := renderWithChild(t, projectSection("T", "D"), `<a id="kid">k</a>`)
		if !strings.HasPrefix(out, "<section") {
			t.Errorf("output starts %q, want a section", out[:min(len(out), 20)])
		}
		assertInOrder(t, out, "<h2", ">T</h2>", ">D</p>", `id="kid"`)
	})
}

func TestProjects(t *testing.T) {
	t.Run("CP-21_the_projects_page_has_a_level_one_heading_and_the_APIary_section", func(t *testing.T) {
		out := renderString(t, Projects())
		assertInOrder(t, out, "<h1", ">Projects</h1>", "<section", ">The APIary</h2>", "</section>")
	})

	t.Run("CP-22_the_section_links_to_the_two_live_demos", func(t *testing.T) {
		out := renderString(t, Projects())
		section := out[strings.Index(out, "<section"):strings.Index(out, "</section>")]
		cards := regexp.MustCompile(`(?s)<a [^>]*>.*?</a>`).FindAllString(section, -1)
		if len(cards) != 2 {
			t.Fatalf("the section holds %d links, want 2", len(cards))
		}
		for i, want := range []struct{ href, title string }{{"/project/bee-name-generator", "Bee Name Generator"}, {"/project/mc-status", "MC Status"}} {
			if href, _ := tagAttr(cards[i], "href"); href != want.href {
				t.Errorf("link %d href = %q, want %q", i, href, want.href)
			}
			assertContains(t, cards[i], ">"+want.title+"</h2>", ">Live</span>")
		}
	})
}

func TestProjectsPage(t *testing.T) {
	t.Run("CP-23_the_projects_page_is_the_projects_content_in_the_default_layout", func(t *testing.T) {
		out := renderString(t, ProjectsPage())
		assertContains(t, out, "<title>NeuralNexus</title>")
		assertInOrder(t, out, "<main", "<h1", ">Projects</h1>", "</main>")
	})
}

func TestBeeNameGenerator(t *testing.T) {
	t.Run("CP-24_the_generate_block_has_the_elements_the_generate_script_writes_to", func(t *testing.T) {
		out := renderString(t, BeeNameGenerator())
		assertContains(t, out, ">Bee Name Generator</h1>")
		assertElement(t, out, "get-bee-name", "h2")
		assertAttr(t, out, "get-bee-name", "aria-live", "polite")
		assertText(t, out, "get-bee-name", "&mdash;")
		assertAttr(t, out, "get-bee-button", "onClick", "getBeeName()")
		assertText(t, out, "get-bee-button", "Generate")
		assertElement(t, out, "get-bee-status", "p")
		assertAttr(t, out, "get-bee-status", "role", "status")
		assertFlag(t, out, "get-bee-status", "hidden", true)
	})

	t.Run("CP-25_the_suggest_form_has_the_input_button_and_status_line_the_suggest_script_uses", func(t *testing.T) {
		out := renderString(t, BeeNameGenerator())
		form := out[strings.Index(out, "<form"):strings.Index(out, "</form>")]
		if got, _ := tagAttr(regexp.MustCompile(`<form[^>]*>`).FindString(form), "onsubmit"); got != "suggestBeeName(event)" {
			t.Errorf("form onsubmit = %q", got)
		}
		assertAttr(t, form, "suggest-bee-name", "name", "name")
		assertAttr(t, form, "suggest-bee-name", "type", "text")
		assertAttr(t, form, "suggest-bee-name", "maxlength", "64")
		assertFlag(t, form, "suggest-bee-name", "required", true)
		assertContains(t, form, `for="suggest-bee-name"`)
		assertAttr(t, form, "suggest-bee-button", "type", "submit")
		assertText(t, form, "suggest-bee-button", "Suggest")
		assertAttr(t, form, "suggest-bee-status", "role", "status")
		assertFlag(t, form, "suggest-bee-status", "hidden", true)
	})

	t.Run("CP-26_the_inline_script_builds_the_two_API_requests", func(t *testing.T) {
		out := renderString(t, BeeNameGenerator())
		script := out[strings.Index(out, "<script"):strings.Index(out, "</script>")]
		assertContains(t, script,
			"fetch(`${apiBaseUrl()}/api/v1/bee-name-generator/name`)",
			"${apiBaseUrl()}/api/v1/bee-name-generator/suggestion/${encodeURIComponent(name)}",
			`method: "POST"`, "input.value.trim()", "isDotSegment(name)")
		assertInOrder(t, script, "isDotSegment(name)", `method: "POST"`)
	})

	t.Run("CP-27_the_page_loads_the_review_link_on_demand", func(t *testing.T) {
		out := renderString(t, BeeNameGenerator())
		tag := regexp.MustCompile(`<div[^>]*data-page-load[^>]*>`).FindString(out)
		for name, want := range map[string]string{"hx-get": "/project/bee-name-generator/admin-link", "hx-trigger": "load", "hx-swap": "outerHTML"} {
			if got, _ := tagAttr(tag, name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
		assertInOrder(t, out, "</form>", tag)
	})
}

func TestBeeNameGeneratorAdminLink(t *testing.T) {
	t.Run("CP-28_the_review_link_points_to_the_suggestion_review_page", func(t *testing.T) {
		out := renderString(t, BeeNameGeneratorAdminLink())
		assertOnce(t, out, "bee-admin-link")
		assertElement(t, out, "bee-admin-link", "a")
		assertAttr(t, out, "bee-admin-link", "href", "/project/bee-name-generator/admin")
		assertText(t, out, "bee-admin-link", "Review suggestions")
	})
}

func TestBeeNameGeneratorPage(t *testing.T) {
	t.Run("CP-29_the_page_has_its_own_title_loads_htmx_and_holds_the_generator", func(t *testing.T) {
		out := renderString(t, BeeNameGeneratorPage())
		assertContains(t, out, "<title>Bee name generator - NeuralNexus</title>")
		assertInOrder(t, out, "<head>", `name="htmx-config"`, "htmx-glue.js", "</head>", "<main", `id="get-bee-button"`, "</main>")
	})
}

func TestBeeNameGeneratorAdmin(t *testing.T) {
	t.Run("CP-30_the_review_shell_shows_its_heading_and_loads_the_suggestions", func(t *testing.T) {
		out := renderString(t, BeeNameGeneratorAdmin())
		assertInOrder(t, out, "<h1", ">Bee Name Suggestions</h1>", "data-page-load")
		tag := regexp.MustCompile(`<div[^>]*data-page-load[^>]*>`).FindString(out)
		for name, want := range map[string]string{"hx-get": "/project/bee-name-generator/admin/suggestions", "hx-trigger": "load", "hx-swap": "outerHTML"} {
			if got, _ := tagAttr(tag, name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
	})
}

func TestBeeSuggestions(t *testing.T) {
	rows := func(t *testing.T, out string) []string {
		t.Helper()
		list, ok := innerHTML(out, "bee-suggestions")
		if !ok {
			t.Fatalf("no bee-suggestions list in:\n%s", out)
		}
		var items []string
		for _, item := range regexp.MustCompile(`(?s)<li[ >].*?</li>`).FindAllString(list, -1) {
			items = append(items, item)
		}
		return items
	}

	t.Run("CP-32_each_name_gets_a_row_with_a_form_that_posts_the_name_and_the_chosen_action", func(t *testing.T) {
		out := renderString(t, BeeSuggestions(BeeSuggestionsData{Names: []string{"Buzz", "Aster", "Buzz"}}))
		assertElement(t, out, "bee-suggestions", "ul")
		items := rows(t, out)
		if len(items) != 3 {
			t.Fatalf("the list holds %d rows, want 3", len(items))
		}
		for i, name := range []string{"Buzz", "Aster", "Buzz"} {
			form := regexp.MustCompile(`<form[^>]*>`).FindString(items[i])
			if got, _ := tagAttr(form, "hx-post"); got != "/project/bee-name-generator/admin/suggestions" {
				t.Errorf("row %d form = %q", i, form)
			}
			hidden := regexp.MustCompile(`<input[^>]*>`).FindString(items[i])
			if kind, _ := tagAttr(hidden, "type"); kind != "hidden" {
				t.Errorf("row %d input = %q, want hidden", i, hidden)
			}
			if field, _ := tagAttr(hidden, "name"); field != "name" {
				t.Errorf("row %d hidden name = %q", i, field)
			}
			if value, _ := tagAttr(hidden, "value"); value != name {
				t.Errorf("row %d hidden value = %q, want %q", i, value, name)
			}
			buttons := regexp.MustCompile(`<button[^>]*>`).FindAllString(items[i], -1)
			if len(buttons) != 2 {
				t.Fatalf("row %d holds %d buttons, want 2", i, len(buttons))
			}
			for j, action := range []string{"accept", "reject"} {
				if got, _ := tagAttr(buttons[j], "name"); got != "action" {
					t.Errorf("row %d button %d name = %q", i, j, got)
				}
				if got, _ := tagAttr(buttons[j], "value"); got != action {
					t.Errorf("row %d button %d value = %q, want %q", i, j, got, action)
				}
			}
			assertInOrder(t, items[i], "<form", hidden, buttons[0], buttons[1])
		}
	})

	t.Run("CP-33_the_root_element_swaps_itself_on_each_response", func(t *testing.T) {
		cases := map[string]BeeSuggestionsData{"with_names": {Names: []string{"Buzz"}}, "without_names": {}}
		for name, data := range cases {
			t.Run("CP-33_"+name, func(t *testing.T) {
				out := renderString(t, BeeSuggestions(data))
				assertAttr(t, out, "bee-suggestions-root", "data-busy-region", "replaced")
				assertAttr(t, out, "bee-suggestions-root", "hx-sync:inherited", "this:drop")
				assertAttr(t, out, "bee-suggestions-root", "hx-target:inherited", "this")
				assertAttr(t, out, "bee-suggestions-root", "hx-swap:inherited", "outerHTML")
			})
		}
	})

	t.Run("CP-34_no_names_shows_the_empty_message_and_no_list_or_forms", func(t *testing.T) {
		for name, data := range map[string]BeeSuggestionsData{"nil": {}, "empty": {Names: []string{}}} {
			t.Run("CP-34_"+name, func(t *testing.T) {
				out := renderString(t, BeeSuggestions(data))
				assertText(t, out, "bee-suggestions-empty", "No pending suggestions")
				assertLacks(t, out, ` id="bee-suggestions"`, "<ul", "<form")
			})
		}
	})

	t.Run("CP-36_names_are_escaped_in_the_text_the_hidden_input_and_the_button_labels", func(t *testing.T) {
		cases := []struct{ name, name_, shown string }{
			{"script", "<script>alert(1)</script>", "&lt;script&gt;alert(1)&lt;/script&gt;"},
			{"quotes_and_markup", `"><b>&'`, "&#34;&gt;&lt;b&gt;&amp;&#39;"},
		}
		for _, tc := range cases {
			t.Run("CP-36_"+tc.name, func(t *testing.T) {
				out := renderString(t, BeeSuggestions(BeeSuggestionsData{Names: []string{tc.name_}}))
				assertLacks(t, out, "<script>alert", "<b>")
				assertContains(t, out, `>`+tc.shown+`</span>`, `name="name" value="`+tc.shown+`"`, `aria-label="Accept `+tc.shown+`"`, `aria-label="Reject `+tc.shown+`"`)
			})
		}
	})

	t.Run("CP-37_the_accept_and_reject_buttons_name_the_suggestion_for_assistive_tech", func(t *testing.T) {
		out := renderString(t, BeeSuggestions(BeeSuggestionsData{Names: []string{"Buzz", "Aster"}}))
		items := rows(t, out)
		for i, name := range []string{"Buzz", "Aster"} {
			assertContains(t, items[i], `aria-label="Accept `+name+`"`, `aria-label="Reject `+name+`"`)
		}
	})

	t.Run("CP-38_the_hidden_input_holds_the_name_exactly_as_given", func(t *testing.T) {
		for _, name := range []string{" Buzz ", "a/b", "Bée 🐝"} {
			t.Run("CP-38_"+strings.TrimSpace(name), func(t *testing.T) {
				out := renderString(t, BeeSuggestions(BeeSuggestionsData{Names: []string{name}}))
				assertContains(t, out, `value="`+name+`"`)
			})
		}
	})
}

func TestBeeNameGeneratorAdminPage(t *testing.T) {
	t.Run("CP-39_the_page_has_its_own_title_loads_htmx_and_holds_the_review_shell", func(t *testing.T) {
		out := renderString(t, BeeNameGeneratorAdminPage())
		assertContains(t, out, "<title>Bee name review - NeuralNexus</title>")
		assertInOrder(t, out, "<head>", "htmx-glue.js", "</head>", "<main", `hx-get="/project/bee-name-generator/admin/suggestions"`, "</main>")
	})
}

func TestMcStatus(t *testing.T) {
	out := ""
	render := func(t *testing.T) string {
		t.Helper()
		out = renderString(t, McStatus())
		return out
	}

	t.Run("CP-40_the_server_form_sends_the_host_and_has_a_submit_button", func(t *testing.T) {
		out := render(t)
		if got, _ := tagAttr(regexp.MustCompile(`<form[^>]*>`).FindString(out), "onsubmit"); got != "checkMcStatus(event)" {
			t.Errorf("form onsubmit = %q", got)
		}
		assertAttr(t, out, "mc-status-host", "name", "host")
		assertAttr(t, out, "mc-status-host", "type", "text")
		assertFlag(t, out, "mc-status-host", "required", true)
		assertAttr(t, out, "mc-status-host", "maxlength", "260")
		assertAttr(t, out, "mc-status-host", "autocomplete", "off")
		if !regexp.MustCompile(`<label[^>]*for="mc-status-host"[^>]*>Server address</label>`).MatchString(out) {
			t.Errorf("no label for mc-status-host reading Server address in:\n%s", out)
		}
		assertAttr(t, out, "mc-status-submit", "type", "submit")
		assertText(t, out, "mc-status-submit", "Check")
	})

	t.Run("CP-41_the_edition_choice_is_two_radios_with_Java_selected", func(t *testing.T) {
		out := render(t)
		radios := regexp.MustCompile(`<input[^>]*type="radio"[^>]*>`).FindAllString(out, -1)
		if len(radios) != 2 {
			t.Fatalf("found %d radios, want 2", len(radios))
		}
		for i, want := range []struct {
			value   string
			checked bool
		}{{"java", true}, {"bedrock", false}} {
			if got, _ := tagAttr(radios[i], "name"); got != "mc-edition" {
				t.Errorf("radio %d name = %q", i, got)
			}
			if got, _ := tagAttr(radios[i], "value"); got != want.value {
				t.Errorf("radio %d value = %q, want %q", i, got, want.value)
			}
			if _, ok := tagAttr(radios[i], "checked"); ok != want.checked {
				t.Errorf("radio %d checked = %t, want %t", i, ok, want.checked)
			}
			if got, _ := tagAttr(radios[i], "onchange"); got != "syncMcStatusQueryOption()" {
				t.Errorf("radio %d onchange = %q", i, got)
			}
		}
	})

	t.Run("CP-42_the_advanced_options_start_closed_with_the_query_checkbox_on_and_a_numeric_port_field", func(t *testing.T) {
		out := render(t)
		assertElement(t, out, "mc-status-advanced", "details")
		assertFlag(t, out, "mc-status-advanced", "open", false)
		assertAttr(t, out, "mc-status-query", "type", "checkbox")
		assertFlag(t, out, "mc-status-query", "checked", true)
		assertAttr(t, out, "mc-status-query-port", "type", "text")
		assertAttr(t, out, "mc-status-query-port", "inputmode", "numeric")
		assertAttr(t, out, "mc-status-query-port", "autocomplete", "off")
	})

	t.Run("CP-43_the_error_region_is_an_alert_that_stays_hidden_until_a_script_shows_it", func(t *testing.T) {
		out := render(t)
		assertAttr(t, out, "mc-status-error", "role", "alert")
		assertFlag(t, out, "mc-status-error", "hidden", true)
		inner, _ := innerHTML(out, "mc-status-error")
		assertContains(t, inner, `id="mc-status-error-message"`, `id="mc-status-error-detail"`)
		assertFlag(t, out, "mc-status-error-detail", "hidden", true)
	})

	t.Run("CP-44_the_result_region_is_hidden_and_carries_the_progress_bar_and_player_list", func(t *testing.T) {
		out := render(t)
		assertAttr(t, out, "mc-status-result", "role", "status")
		assertFlag(t, out, "mc-status-result", "hidden", true)
		assertFlag(t, out, "mc-status-icon", "hidden", true)
		assertAttr(t, out, "mc-status-icon", "alt", "")
		bar := regexp.MustCompile(`<div[^>]*role="progressbar"[^>]*>`).FindString(out)
		for name, want := range map[string]string{"aria-label": "Players online", "aria-valuemin": "0", "aria-valuenow": "0", "aria-valuemax": "0"} {
			if got, _ := tagAttr(bar, name); got != want {
				t.Errorf("progressbar %s = %q, want %q", name, got, want)
			}
		}
		assertFlag(t, out, "mc-status-players-unavailable", "hidden", true)
		assertText(t, out, "mc-status-players-unavailable", "Player list unavailable")
	})

	t.Run("CP-45_every_id_the_status_script_writes_to_or_reads_from_exists_once", func(t *testing.T) {
		out := render(t)
		for _, id := range []string{
			"mc-status-host", "mc-status-query", "mc-status-query-port", "mc-status-submit", "mc-status-advanced", "mc-status-name",
			"mc-status-motd", "mc-status-version", "mc-status-type", "mc-status-pill", "mc-status-players-count", "mc-status-players-bar",
			"mc-status-icon", "mc-status-players", "mc-status-players-unavailable", "mc-status-error", "mc-status-error-message",
			"mc-status-error-detail", "mc-status-result",
		} {
			t.Run("CP-45_"+id, func(t *testing.T) { assertOnce(t, out, id) })
		}
	})

	t.Run("CP-46_the_page_starts_the_lookup_from_the_URL_after_the_form", func(t *testing.T) {
		out := render(t)
		if script := lastScript(t, out); !strings.Contains(script, "loadMcStatusFromUrl();") || strings.LastIndex(out, "<script>") < strings.Index(out, `id="mc-status-result"`) {
			t.Errorf("last script = %q, want loadMcStatusFromUrl(); after mc-status-result", script)
		}
	})
}

var accountPlatformNames = []struct{ id, name string }{
	{"discord", "Discord"},
	{"twitch", "Twitch"},
	{"microsoft", "Microsoft"},
	{"xboxlive", "Xbox Live"},
	{"steam", "Steam"},
}

func TestAccount(t *testing.T) {
	t.Run("CP-48_the_shell_has_its_heading_and_loads_the_account_content", func(t *testing.T) {
		out := renderString(t, Account())
		assertContains(t, out, ">Account Settings</h1>")
		tag := regexp.MustCompile(`<div[^>]*data-page-load[^>]*>`).FindString(out)
		for name, want := range map[string]string{"hx-get": "/account/content", "hx-trigger": "load", "hx-swap": "outerHTML"} {
			if got, _ := tagAttr(tag, name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
	})

	t.Run("CP-49_the_shell_holds_the_auth_error_banner_the_error_and_status_regions_and_the_error_script", func(t *testing.T) {
		out := renderString(t, Account())
		assertFlag(t, out, "auth-error", "hidden", true)
		assertAttr(t, out, "page-error", "role", "alert")
		assertAttr(t, out, "page-status", "role", "status")
		if script := lastScript(t, out); !strings.Contains(script, "showAuthErrorFromQuery();") {
			t.Errorf("last script = %q", script)
		}
	})

	t.Run("CP-50_the_shell_includes_the_link_setup", func(t *testing.T) {
		out := renderString(t, Account())
		assertContains(t, out, "const linkRedirect = window.location.href;")
		for _, id := range []string{"link-discord-oauth-base", "link-twitch-oauth-base", "link-microsoft-oauth-base", "link-xboxlive-oauth-base"} {
			assertOnce(t, out, id)
		}
	})
}

func TestAccountContent(t *testing.T) {
	t.Run("CP-51_the_username_is_shown_and_escaped", func(t *testing.T) {
		out := renderString(t, AccountContent(AccountData{Username: "<b>a&b</b>"}))
		assertText(t, out, "account-username", "&lt;b&gt;a&amp;b&lt;/b&gt;")
		assertLacks(t, out, "<b>a")
	})

	t.Run("CP-55_a_links_error_replaces_the_rows_and_is_escaped", func(t *testing.T) {
		out := renderString(t, AccountContent(AccountData{LinksError: "<i>links down</i>", Links: map[string]LinkedAccount{"discord": {PlatformUsername: "alice"}}}))
		assertAttr(t, out, "account-links-error", "role", "alert")
		assertText(t, out, "account-links-error", "&lt;i&gt;links down&lt;/i&gt;")
		assertLacks(t, out, ` id="account-links"`, `id="link-discord`)
	})
}

func TestAccountDataHasLink(t *testing.T) {
	t.Run("CP-58_a_platform_is_linked_when_its_key_is_present_whatever_the_entry_holds", func(t *testing.T) {
		cases := []struct {
			name  string
			links map[string]LinkedAccount
			check map[string]bool
		}{
			{"zero_entry", map[string]LinkedAccount{"discord": {}}, map[string]bool{"discord": true}},
			{"other_keys", map[string]LinkedAccount{"discord": {}}, map[string]bool{"steam": false, "Discord": false, "": false}},
			{"nil_map", nil, map[string]bool{"discord": false}},
		}
		for _, tc := range cases {
			t.Run("CP-58_"+tc.name, func(t *testing.T) {
				for platform, want := range tc.check {
					if got := (AccountData{Links: tc.links}).HasLink(platform); got != want {
						t.Errorf("HasLink(%q) = %t, want %t", platform, got, want)
					}
				}
			})
		}
	})
}

func TestAccountPasswordLogin(t *testing.T) {
	t.Run("CP-60_the_setting_is_a_switch_checkbox_that_posts_its_field_on_change", func(t *testing.T) {
		out := renderString(t, AccountPasswordLogin(AccountSettingsData{}, false))
		for name, want := range map[string]string{"type": "checkbox", "name": "password_auth", "value": "true", "hx-post": "/account/settings", "hx-trigger": "change", "role": "switch"} {
			assertAttr(t, out, "password-auth-enabled", name, want)
		}
		if !regexp.MustCompile(`(?s)<label[^>]*for="password-auth-enabled"[^>]*>\s*Password login\s*</label>`).MatchString(out) {
			t.Errorf("no label for password-auth-enabled reading Password login in:\n%s", out)
		}
	})

	t.Run("CP-61_checked_and_disabled_follow_PasswordAuth_and_Error", func(t *testing.T) {
		cases := []struct {
			name     string
			data     AccountSettingsData
			checked  bool
			disabled bool
		}{
			{"on_without_error", AccountSettingsData{PasswordAuth: true}, true, false},
			{"off_without_error", AccountSettingsData{}, false, false},
			{"on_with_error", AccountSettingsData{PasswordAuth: true, Error: "x"}, true, true},
			{"off_with_error", AccountSettingsData{Error: "x"}, false, true},
		}
		for _, tc := range cases {
			t.Run("CP-61_"+tc.name, func(t *testing.T) {
				out := renderString(t, AccountPasswordLogin(tc.data, false))
				assertFlag(t, out, "password-auth-enabled", "checked", tc.checked)
				assertFlag(t, out, "password-auth-enabled", "disabled", tc.disabled)
				if got := strings.Contains(out, ` id="account-password-error"`); got != tc.disabled {
					t.Errorf("error element present = %t, want %t", got, tc.disabled)
				}
			})
		}
	})

	t.Run("CP-62_the_error_text_is_escaped_and_announced", func(t *testing.T) {
		out := renderString(t, AccountPasswordLogin(AccountSettingsData{Error: "<i>x</i>"}, false))
		assertAttr(t, out, "account-password-error", "role", "alert")
		assertText(t, out, "account-password-error", "&lt;i&gt;x&lt;/i&gt;")
	})

	t.Run("CP-64_the_root_swaps_itself_one_request_at_a_time", func(t *testing.T) {
		for _, oob := range []bool{true, false} {
			t.Run("CP-64_"+map[bool]string{true: "oob", false: "in_place"}[oob], func(t *testing.T) {
				out := renderString(t, AccountPasswordLogin(AccountSettingsData{}, oob))
				assertAttr(t, out, "account-password", "data-busy-region", "replaced")
				assertAttr(t, out, "account-password", "hx-sync:inherited", "this:drop")
				assertAttr(t, out, "account-password", "hx-target:inherited", "this")
				assertAttr(t, out, "account-password", "hx-swap:inherited", "outerHTML")
			})
		}
	})
}

func TestAccountLinkRow(t *testing.T) {
	discord := AccountPlatform{ID: "discord", Name: "Discord"}

	t.Run("CP-67_an_unlinked_platform_shows_its_name_and_a_Link_button_that_starts_the_browser_flow", func(t *testing.T) {
		for _, p := range accountPlatformNames {
			t.Run("CP-67_"+p.id, func(t *testing.T) {
				out := renderString(t, AccountLinkRow(AccountPlatform{ID: p.id, Name: p.name}, LinkedAccount{}, false, false))
				base := "link-" + p.id
				assertOnce(t, out, base)
				assertText(t, out, base+"-title", p.name)
				action := base + "-action"
				assertAttr(t, out, action, "type", "button")
				assertAttr(t, out, action, "data-platform", p.id)
				assertAttr(t, out, action, "onclick", "handleLinkAction(this.dataset.platform)")
				assertAttr(t, out, action, "aria-label", "Link "+p.name)
				assertText(t, out, action, "Link")
				assertFlag(t, out, action, "hx-delete", false)
				assertFlag(t, out, action, "hx-confirm", false)
				assertHasClass(t, out, base+"-subtitle", "hidden", true)
			})
		}
	})

	t.Run("CP-68_a_linked_verified_platform_shows_the_username_and_an_Unlink_button_that_sends_a_confirmed_DELETE", func(t *testing.T) {
		for _, p := range accountPlatformNames {
			t.Run("CP-68_"+p.id, func(t *testing.T) {
				out := renderString(t, AccountLinkRow(AccountPlatform{ID: p.id, Name: p.name}, LinkedAccount{PlatformUsername: "alice", Verified: true}, true, false))
				base := "link-" + p.id
				assertText(t, out, base+"-title", "alice")
				action := base + "-action"
				assertAttr(t, out, action, "hx-delete", "/account/links/"+p.id)
				assertAttr(t, out, action, "hx-confirm", "Unlink "+p.name+" from your account?")
				assertAttr(t, out, action, "aria-label", "Unlink "+p.name)
				assertText(t, out, action, "Unlink")
				assertFlag(t, out, action, "data-platform", false)
				assertFlag(t, out, action, "onclick", false)
				assertHasClass(t, out, base+"-subtitle", "hidden", false)
				assertText(t, out, base+"-subtitle", p.name)
				assertHasClass(t, out, base+"-verified-icon", "hidden", false)
				assertText(t, out, base+"-status", "")
			})
		}
	})

	t.Run("CP-69_a_linked_platform_that_is_not_verified_says_so_hides_the_check_mark_and_disables_the_login_checkbox", func(t *testing.T) {
		out := renderString(t, AccountLinkRow(discord, LinkedAccount{PlatformUsername: "alice", Verified: false, LoginEnabled: true}, true, false))
		assertText(t, out, "link-discord-status", "Unverified")
		assertHasClass(t, out, "link-discord-verified-icon", "hidden", true)
		assertFlag(t, out, "link-discord-login-enabled", "disabled", true)
	})

	t.Run("CP-70_a_linked_platform_without_a_username_shows_the_platform_name", func(t *testing.T) {
		out := renderString(t, AccountLinkRow(discord, LinkedAccount{Verified: true}, true, false))
		assertText(t, out, "link-discord-title", "Discord")
	})

	t.Run("CP-71_an_unlinked_platform_ignores_the_data_of_the_link_it_was_given", func(t *testing.T) {
		out := renderString(t, AccountLinkRow(discord, LinkedAccount{PlatformUsername: "stale", Verified: true, LoginEnabled: true}, false, false))
		assertText(t, out, "link-discord-title", "Discord")
		assertLacks(t, out, "stale")
		assertText(t, out, "link-discord-status", "")
		assertHasClass(t, out, "link-discord-verified-icon", "hidden", true)
		assertFlag(t, out, "link-discord-login-enabled", "disabled", true)
		assertFlag(t, out, "link-discord-login-enabled", "checked", false)
		assertFlag(t, out, "link-discord-action", "data-platform", true)
	})

	t.Run("CP-72_the_login_checkbox_state_follows_linked_verified_and_login_enabled", func(t *testing.T) {
		cases := []struct {
			name                    string
			linked, verified, login bool
			checked, disabled       bool
		}{
			{"linked_verified_enabled", true, true, true, true, false},
			{"linked_verified_disabled", true, true, false, false, false},
			{"linked_unverified_enabled", true, false, true, true, true},
			{"linked_unverified_disabled", true, false, false, false, true},
			{"unlinked", false, true, true, false, true},
		}
		for _, tc := range cases {
			t.Run("CP-72_"+tc.name, func(t *testing.T) {
				out := renderString(t, AccountLinkRow(discord, LinkedAccount{PlatformUsername: "alice", Verified: tc.verified, LoginEnabled: tc.login}, tc.linked, false))
				assertFlag(t, out, "link-discord-login-enabled", "checked", tc.checked)
				assertFlag(t, out, "link-discord-login-enabled", "disabled", tc.disabled)
			})
		}
	})

	t.Run("CP-73_the_out_of_band_flag_adds_hx_swap_oob_to_the_root_and_the_root_swaps_itself", func(t *testing.T) {
		for _, oob := range []bool{true, false} {
			t.Run("CP-73_"+map[bool]string{true: "oob", false: "in_place"}[oob], func(t *testing.T) {
				out := renderString(t, AccountLinkRow(discord, LinkedAccount{}, false, oob))
				if oob {
					assertAttr(t, out, "link-discord", "hx-swap-oob", "true")
				} else {
					assertFlag(t, out, "link-discord", "hx-swap-oob", false)
				}
				assertAttr(t, out, "link-discord", "data-busy-region", "replaced")
				assertAttr(t, out, "link-discord", "hx-sync:inherited", "this:drop")
				assertAttr(t, out, "link-discord", "hx-target:inherited", "this")
				assertAttr(t, out, "link-discord", "hx-swap:inherited", "outerHTML")
			})
		}
	})

	t.Run("CP-74_a_username_the_API_returned_is_escaped", func(t *testing.T) {
		cases := []struct{ name, username, shown string }{
			{"image_tag", "<img src=x onerror=alert(1)>", "&lt;img src=x onerror=alert(1)&gt;"},
			{"quotes", `"a"&'b'`, "&#34;a&#34;&amp;&#39;b&#39;"},
		}
		for _, tc := range cases {
			t.Run("CP-74_"+tc.name, func(t *testing.T) {
				out := renderString(t, AccountLinkRow(discord, LinkedAccount{PlatformUsername: tc.username, Verified: true}, true, false))
				assertText(t, out, "link-discord-title", tc.shown)
				assertLacks(t, out, "<img src=x")
			})
		}
	})

	t.Run("CP-75_the_login_label_points_at_the_login_checkbox", func(t *testing.T) {
		out := renderString(t, AccountLinkRow(discord, LinkedAccount{}, true, false))
		if !regexp.MustCompile(`(?s)<label[^>]*for="link-discord-login-enabled"[^>]*>\s*Allow logins\s*</label>`).MatchString(out) {
			t.Errorf("no label for the login checkbox reading Allow logins in:\n%s", out)
		}
		assertOnce(t, out, "link-discord-login-enabled")
	})
}

func TestAccountLoginInput(t *testing.T) {
	t.Run("CP-76_the_checkbox_is_a_switch_that_posts_to_its_platform_on_change", func(t *testing.T) {
		for _, p := range accountPlatformNames {
			t.Run("CP-76_"+p.id, func(t *testing.T) {
				out := renderString(t, AccountLoginInput(AccountPlatform{ID: p.id, Name: p.name}, false, false, false))
				id := "link-" + p.id + "-login-enabled"
				for name, want := range map[string]string{
					"type": "checkbox", "name": "login_enabled", "value": "true", "hx-post": "/account/links/" + p.id,
					"hx-trigger": "change", "role": "switch", "aria-label": "Allow logins for " + p.name,
				} {
					assertAttr(t, out, id, name, want)
				}
			})
		}
	})

	t.Run("CP-77_checked_and_disabled_follow_their_arguments", func(t *testing.T) {
		for _, tc := range []struct{ checked, disabled bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
			t.Run("CP-77_"+map[bool]string{true: "checked", false: "unchecked"}[tc.checked]+"_"+map[bool]string{true: "disabled", false: "enabled"}[tc.disabled], func(t *testing.T) {
				out := renderString(t, AccountLoginInput(AccountPlatform{ID: "discord", Name: "Discord"}, tc.checked, tc.disabled, false))
				assertFlag(t, out, "link-discord-login-enabled", "checked", tc.checked)
				assertFlag(t, out, "link-discord-login-enabled", "disabled", tc.disabled)
			})
		}
	})

	t.Run("CP-78_the_out_of_band_flag_adds_hx_swap_oob_to_the_checkbox", func(t *testing.T) {
		for _, oob := range []bool{true, false} {
			t.Run("CP-78_"+map[bool]string{true: "oob", false: "in_place"}[oob], func(t *testing.T) {
				out := renderString(t, AccountLoginInput(AccountPlatform{ID: "discord", Name: "Discord"}, false, false, oob))
				if oob {
					assertAttr(t, out, "link-discord-login-enabled", "hx-swap-oob", "true")
				} else {
					assertFlag(t, out, "link-discord-login-enabled", "hx-swap-oob", false)
				}
			})
		}
	})
}

func TestAccountPlatformIcon(t *testing.T) {
	images := func(out string) []string { return regexp.MustCompile(`<img[^>]*>`).FindAllString(out, -1) }
	check := func(t *testing.T, img, src, alt string) {
		t.Helper()
		if got, _ := tagAttr(img, "src"); got != src {
			t.Errorf("img %s src = %q, want %q", img, got, src)
		}
		if got, _ := tagAttr(img, "alt"); got != alt {
			t.Errorf("img %s alt = %q, want %q", img, got, alt)
		}
	}

	t.Run("CP-79_Microsoft_shows_its_logo_image", func(t *testing.T) {
		imgs := images(renderString(t, accountPlatformIcon("microsoft")))
		if len(imgs) != 1 {
			t.Fatalf("found %d images, want 1", len(imgs))
		}
		check(t, imgs[0], "/public/MicrosoftLogo.svg", "Microsoft Logo")
	})

	t.Run("CP-80_Steam_shows_a_light_and_a_dark_logo_image", func(t *testing.T) {
		imgs := images(renderString(t, accountPlatformIcon("steam")))
		if len(imgs) != 2 {
			t.Fatalf("found %d images, want 2", len(imgs))
		}
		check(t, imgs[0], "/public/SteamLogo.svg", "Steam Logo")
		check(t, imgs[1], "/public/SteamLogoWhite.svg", "Steam Logo")
	})

	t.Run("CP-81_Minecraft_shows_its_logo_image", func(t *testing.T) {
		imgs := images(renderString(t, accountPlatformIcon("minecraft")))
		if len(imgs) != 1 {
			t.Fatalf("found %d images, want 1", len(imgs))
		}
		check(t, imgs[0], "/public/MinecraftLogo.svg", "Minecraft Logo")
	})

	t.Run("CP-82_the_other_OAuth_providers_show_the_logo_from_the_provider_table", func(t *testing.T) {
		cases := []struct{ platform, src, alt string }{
			{"discord", "/public/DiscordClydeWhite.svg", "Discord Logo"},
			{"twitch", "/public/TwitchGlitchWhite.svg", "Twitch Logo"},
			{"xboxlive", "/public/XboxSymbolWhite.svg", "Xbox Logo"},
		}
		for _, tc := range cases {
			t.Run("CP-82_"+tc.platform, func(t *testing.T) {
				imgs := images(renderString(t, accountPlatformIcon(tc.platform)))
				if len(imgs) != 1 {
					t.Fatalf("found %d images, want 1", len(imgs))
				}
				check(t, imgs[0], tc.src, tc.alt)
			})
		}
	})

	t.Run("CP-83_an_unknown_platform_renders_nothing", func(t *testing.T) {
		for name, platform := range map[string]string{"unknown": "nope", "empty": "", "capitalised": "Discord"} {
			t.Run("CP-83_"+name, func(t *testing.T) {
				if out := renderString(t, accountPlatformIcon(platform)); strings.TrimSpace(out) != "" {
					t.Errorf("output = %q, want empty", out)
				}
			})
		}
	})
}

func TestAccountPage(t *testing.T) {
	t.Run("CP-84_the_page_has_its_own_title_loads_htmx_and_holds_the_account_shell", func(t *testing.T) {
		out := renderString(t, AccountPage())
		assertContains(t, out, "<title>Account - NeuralNexus</title>")
		assertInOrder(t, out, "<head>", "htmx-glue.js", "</head>", "<main", `hx-get="/account/content"`, "</main>")
	})
}

func TestLoginButton(t *testing.T) {
	t.Run("CP-85_the_account_logout_and_login_controls_all_start_hidden", func(t *testing.T) {
		out := renderString(t, LoginButton())
		for _, id := range []string{"header-account", "header-logout-btn", "header-login-btn"} {
			assertOnce(t, out, id)
			assertFlag(t, out, id, "hidden", true)
		}
	})

	t.Run("CP-86_the_account_control_has_a_username_slot_and_a_labelled_settings_link", func(t *testing.T) {
		out := renderString(t, LoginButton())
		inner, _ := innerHTML(out, "header-account")
		assertText(t, inner, "header-account-username", "")
		link := regexp.MustCompile(`<a[^>]*>`).FindString(inner)
		if got, _ := tagAttr(link, "href"); got != "/account" {
			t.Errorf("link = %q, want href /account", link)
		}
		if got, _ := tagAttr(link, "aria-label"); got != "Account settings" {
			t.Errorf("link = %q, want aria-label Account settings", link)
		}
		assertInOrder(t, inner, `id="header-account-username"`, link)
	})

	t.Run("CP-87_the_logout_control_is_a_button_that_calls_logout", func(t *testing.T) {
		out := renderString(t, LoginButton())
		inner, _ := innerHTML(out, "header-logout-btn")
		button := regexp.MustCompile(`<button[^>]*>`).FindString(inner)
		if got, _ := tagAttr(button, "onclick"); got != "logout()" {
			t.Errorf("button = %q, want onclick logout()", button)
		}
		if !regexp.MustCompile(`(?s)>\s*Logout\s*</button>`).MatchString(inner) {
			t.Errorf("the button does not read Logout: %q", inner)
		}
	})

	t.Run("CP-88_the_login_control_is_a_link_to_the_login_page", func(t *testing.T) {
		out := renderString(t, LoginButton())
		inner, _ := innerHTML(out, "header-login-btn")
		link := regexp.MustCompile(`<a[^>]*>`).FindString(inner)
		if got, _ := tagAttr(link, "href"); got != "/login" {
			t.Errorf("link = %q, want href /login", link)
		}
		if !regexp.MustCompile(`(?s)>\s*Login\s*</a>`).MatchString(inner) {
			t.Errorf("the link does not read Login: %q", inner)
		}
		assertLacks(t, inner, "<button")
	})

	t.Run("CP-89_the_controls_are_followed_by_the_script_that_checks_the_sign_in_state", func(t *testing.T) {
		out := renderString(t, LoginButton())
		if n := strings.Count(out, "checkHeaderAuthState();"); n != 1 {
			t.Errorf("checkHeaderAuthState(); appears %d times, want 1", n)
		}
		assertInOrder(t, out, `id="header-login-btn"`, "<script>", "checkHeaderAuthState();")
	})
}

func TestOAuthProviderFor(t *testing.T) {
	t.Run("CP-90_each_provider_id_returns_its_provider_and_any_other_id_returns_the_zero_value", func(t *testing.T) {
		known := []struct{ id, element, label, icon, alt string }{
			{"discord", "discord-oauth", "Discord", "/public/DiscordClydeWhite.svg", "Discord Logo"},
			{"twitch", "twitch-oauth", "Twitch", "/public/TwitchGlitchWhite.svg", "Twitch Logo"},
			{"microsoft", "microsoft-oauth", "Microsoft", "/public/MicrosoftLogo.svg", "Microsoft Logo"},
			{"xboxlive", "xbox-oauth", "Xbox Live", "/public/XboxSymbolWhite.svg", "Xbox Logo"},
		}
		for _, tc := range known {
			t.Run("CP-90_"+tc.id, func(t *testing.T) {
				got, ok := oauthProviderFor(tc.id)
				if !ok || got.ID != tc.id || got.ElementID != tc.element || got.Label != tc.label || got.Icon != tc.icon || got.Alt != tc.alt {
					t.Errorf("oauthProviderFor(%q) = %+v, %t", tc.id, got, ok)
				}
			})
		}
		for name, id := range map[string]string{"empty": "", "steam": "steam", "minecraft": "minecraft", "capitalised": "Discord", "trailing_space": "discord "} {
			t.Run("CP-90_unknown_"+name, func(t *testing.T) {
				if got, ok := oauthProviderFor(id); ok || got != (oauthProvider{}) {
					t.Errorf("oauthProviderFor(%q) = %+v, %t, want the zero value and false", id, got, ok)
				}
			})
		}
	})
}

func TestOAuthBases(t *testing.T) {
	providers := []string{"discord", "twitch", "microsoft", "xboxlive"}

	t.Run("CP-92_each_provider_gets_a_hidden_element_whose_id_carries_the_prefix", func(t *testing.T) {
		for name, prefix := range map[string]string{"no_prefix": "", "link_prefix": "link-"} {
			t.Run("CP-92_"+name, func(t *testing.T) {
				out := renderString(t, oauthBases(prefix))
				var ids []string
				for _, provider := range providers {
					id := prefix + provider + "-oauth-base"
					assertOnce(t, out, id)
					assertFlag(t, out, id, "hidden", true)
					ids = append(ids, `id="`+id+`"`)
				}
				assertInOrder(t, out, ids...)
				if prefix == "" {
					assertLacks(t, out, `id="link-`)
				}
			})
		}
	})

	t.Run("CP-93_each_element_holds_the_providers_authorize_URL_escaped_as_text", func(t *testing.T) {
		cases := []struct{ provider, want string }{
			{"discord", "https://discord.com/api/oauth2/authorize?client_id=" + DISCORD_CLIENT_ID + "&redirect_uri=" + DISCORD_REDIRECT_URI + "&response_type=code&scope=identify%20email"},
			{"twitch", "https://id.twitch.tv/oauth2/authorize?client_id=" + TWITCH_CLIENT_ID + "&redirect_uri=" + TWITCH_REDIRECT_URI + "&response_type=code&scope=user%3Aread%3Aemail"},
			{"microsoft", "https://login.microsoftonline.com/consumers/oauth2/v2.0/authorize?client_id=" + MICROSOFT_CLIENT_ID + "&redirect_uri=" + MICROSOFT_REDIRECT_URI + "&response_type=code&scope=openid%20profile%20email%20offline_access"},
			{"xboxlive", "https://login.microsoftonline.com/consumers/oauth2/v2.0/authorize?client_id=" + MICROSOFT_CLIENT_ID + "&redirect_uri=" + MICROSOFT_REDIRECT_URI + "&response_type=code&scope=XboxLive.signin%20offline_access"},
		}
		out := renderString(t, oauthBases(""))
		for _, tc := range cases {
			t.Run("CP-93_"+tc.provider, func(t *testing.T) {
				inner, ok := innerHTML(out, tc.provider+"-oauth-base")
				if !ok {
					t.Fatalf("no element for %s in:\n%s", tc.provider, out)
				}
				if got := html.UnescapeString(strings.TrimSpace(inner)); got != tc.want {
					t.Errorf("text = %q, want %q", got, tc.want)
				}
			})
		}
		assertContains(t, out, "&amp;response_type=code")
		if strings.Contains(strings.ReplaceAll(out, "&amp;", ""), "&response_type") {
			t.Errorf("the output holds a bare &response_type:\n%s", out)
		}
	})
}

func TestOAuthButtons(t *testing.T) {
	providers := []struct{ id, element, label, icon, alt string }{
		{"discord", "discord-oauth", "Discord", "/public/DiscordClydeWhite.svg", "Discord Logo"},
		{"twitch", "twitch-oauth", "Twitch", "/public/TwitchGlitchWhite.svg", "Twitch Logo"},
		{"microsoft", "microsoft-oauth", "Microsoft", "/public/MicrosoftLogo.svg", "Microsoft Logo"},
		{"xboxlive", "xbox-oauth", "Xbox Live", "/public/XboxSymbolWhite.svg", "Xbox Logo"},
	}

	t.Run("CP-94_each_provider_has_an_anchor_that_starts_the_login_for_its_own_provider", func(t *testing.T) {
		out := renderString(t, OAuthButtons("Login with"))
		var ids []string
		for _, p := range providers {
			ids = append(ids, `id="`+p.element+`"`)
		}
		assertInOrder(t, out, ids...)
		for _, p := range providers {
			t.Run("CP-94_"+p.id, func(t *testing.T) {
				assertAttr(t, out, p.element, "data-provider", p.id)
				assertAttr(t, out, p.element, "onclick", "startOAuthLogin(this.dataset.provider, document.getElementById(this.dataset.provider + '-oauth-base').innerText); return false;")
			})
		}
	})

	t.Run("CP-95_each_button_label_joins_the_prefix_and_the_provider_name_and_shows_the_provider_logo", func(t *testing.T) {
		for _, prefix := range []string{"Login with", "Sign up with"} {
			out := renderString(t, OAuthButtons(prefix))
			for _, p := range providers {
				t.Run("CP-95_"+strings.ReplaceAll(prefix, " ", "_")+"_"+p.id, func(t *testing.T) {
					tag := testutil.TagByID(out, p.element)
					block := out[strings.Index(out, tag):]
					block = block[:strings.Index(block, "</a>")]
					assertContains(t, block, `<span class="mr-2 text-white">`+prefix+" "+p.label+"</span>")
					img := regexp.MustCompile(`<img[^>]*>`).FindString(block)
					if got, _ := tagAttr(img, "src"); got != p.icon {
						t.Errorf("img src = %q, want %q", got, p.icon)
					}
					if got, _ := tagAttr(img, "alt"); got != p.alt {
						t.Errorf("img alt = %q, want %q", got, p.alt)
					}
				})
			}
		}
	})

	t.Run("CP-96_every_provider_anchor_has_the_hidden_element_it_reads", func(t *testing.T) {
		out := renderString(t, OAuthButtons("Login with"))
		for _, p := range providers {
			t.Run("CP-96_"+p.id, func(t *testing.T) {
				value, _ := tagAttr(testutil.TagByID(out, p.element), "data-provider")
				assertFlag(t, out, value+"-oauth-base", "hidden", true)
			})
		}
	})

	t.Run("CP-97_Steam_has_its_own_sign_in_link_with_no_base_element", func(t *testing.T) {
		out := renderString(t, OAuthButtons("Login with"))
		assertAttr(t, out, "steam-oauth", "onclick", "startOAuthLogin('steam', null); return false;")
		inner, _ := innerHTML(out, "steam-oauth")
		img := regexp.MustCompile(`<img[^>]*>`).FindString(inner)
		if got, _ := tagAttr(img, "src"); got != "/public/SteamSignIn.png" {
			t.Errorf("img src = %q", got)
		}
		if got, _ := tagAttr(img, "alt"); got != "Sign in through Steam" {
			t.Errorf("img alt = %q", got)
		}
		assertLacks(t, out, "steam-oauth-base")
	})
}

func TestLinkAccountSetup(t *testing.T) {
	t.Run("CP-98_the_setup_stores_the_page_address_and_has_the_link_prefixed_bases", func(t *testing.T) {
		out := renderString(t, LinkAccountSetup())
		assertContains(t, out, "const linkRedirect = window.location.href;")
		for _, id := range []string{"link-discord-oauth-base", "link-twitch-oauth-base", "link-microsoft-oauth-base", "link-xboxlive-oauth-base"} {
			assertFlag(t, out, id, "hidden", true)
		}
		assertLacks(t, out, ` id="discord-oauth-base"`, ` id="twitch-oauth-base"`, ` id="microsoft-oauth-base"`, ` id="xboxlive-oauth-base"`)
	})
}

func TestLogin(t *testing.T) {
	t.Run("CP-99_the_login_page_has_its_heading_the_error_banner_and_the_error_script", func(t *testing.T) {
		out := renderString(t, Login())
		assertContains(t, out, ">Login</h1>")
		assertFlag(t, out, "auth-error", "hidden", true)
		if script := lastScript(t, out); !strings.Contains(script, "showAuthErrorFromQuery();") {
			t.Errorf("last script = %q", script)
		}
	})

	t.Run("CP-100_the_login_form_has_the_fields_the_submit_script_reads", func(t *testing.T) {
		out := renderString(t, Login())
		form := regexp.MustCompile(`<form[^>]*>`).FindString(out)
		if got, _ := tagAttr(form, "onSubmit"); got != "submitLoginForm()" {
			t.Errorf("form = %q, want onSubmit submitLoginForm()", form)
		}
		assertAttr(t, out, "username", "name", "username")
		assertAttr(t, out, "username", "type", "text")
		assertAttr(t, out, "password", "name", "password")
		assertAttr(t, out, "password", "type", "password")
		assertContains(t, out, `for="username"`, `for="password"`)
		if !regexp.MustCompile(`(?s)<button[^>]*type="submit"[^>]*>\s*Login\s*</button>`).MatchString(out) {
			t.Errorf("no submit button reading Login in:\n%s", out)
		}
	})

	t.Run("CP-101_the_page_offers_the_OAuth_buttons_labelled_for_login_and_a_link_to_sign_up", func(t *testing.T) {
		out := renderString(t, Login())
		assertOnce(t, out, "discord-oauth")
		assertOnce(t, out, "steam-oauth")
		assertContains(t, out, `>Login with Discord</span>`, `href="/register"`)
		assertLacks(t, out, "Sign up with")
	})
}

func TestLoginPage(t *testing.T) {
	t.Run("CP-102_the_login_page_is_the_login_form_in_the_default_layout", func(t *testing.T) {
		out := renderString(t, LoginPage())
		assertContains(t, out, "<title>NeuralNexus</title>")
		assertInOrder(t, out, "<main", `id="username"`, "</main>")
	})
}

func TestRegister(t *testing.T) {
	t.Run("CP-103_the_sign_up_page_has_its_heading_the_error_banner_and_the_error_script", func(t *testing.T) {
		out := renderString(t, Register())
		assertContains(t, out, ">Sign Up</h1>")
		assertFlag(t, out, "auth-error", "hidden", true)
		if script := lastScript(t, out); !strings.Contains(script, "showAuthErrorFromQuery();") {
			t.Errorf("last script = %q", script)
		}
	})

	t.Run("CP-106_the_page_offers_the_OAuth_buttons_labelled_for_sign_up_and_a_link_to_login", func(t *testing.T) {
		out := renderString(t, Register())
		assertOnce(t, out, "discord-oauth")
		assertOnce(t, out, "steam-oauth")
		assertContains(t, out, `>Sign up with Discord</span>`, `href="/login"`)
		assertLacks(t, out, "Login with")
	})
}

func TestRegisterPage(t *testing.T) {
	t.Run("CP-107_the_sign_up_page_is_the_sign_up_form_in_the_default_layout", func(t *testing.T) {
		out := renderString(t, RegisterPage())
		assertContains(t, out, "<title>NeuralNexus</title>")
		assertInOrder(t, out, "<main", `id="confirm_password"`, "</main>")
	})
}

func TestTeapotPage(t *testing.T) {
	t.Run("CP-108_the_teapot_page_is_the_teapot_content_in_the_default_layout", func(t *testing.T) {
		out := renderString(t, TeapotPage())
		assertContains(t, out, "<title>NeuralNexus</title>")
		assertInOrder(t, out, "<main", `alt="Teapot"`, "</main>")
	})
}

func TestTeapot(t *testing.T) {
	t.Run("CP-109_the_preview_meta_tags_describe_the_teapot_and_use_the_site_URL", func(t *testing.T) {
		out := renderString(t, Teapot())
		cases := []struct{ key, want string }{
			{"og:site_name", "Powered by NeuralNexus.dev"},
			{"og:title", "418 I'm a teapot"},
			{"og:description", "You requested a cup of coffee, but I'm a teapot."},
			{"og:url", "http://site.test/teapot"},
			{"og:image", "http://site.test/public/teapot.jpg"},
			{"theme-color", "#7C0014"},
		}
		for _, tc := range cases {
			t.Run("CP-109_"+strings.ReplaceAll(tc.key, ":", "_"), func(t *testing.T) {
				tag := regexp.MustCompile(`<meta [^>]*(?:property|name)="` + regexp.QuoteMeta(tc.key) + `"[^>]*>`).FindString(out)
				if got, _ := tagAttr(tag, "content"); got != tc.want {
					t.Errorf("%s content = %q, want %q; tag %q", tc.key, got, tc.want, tag)
				}
			})
		}
	})

	t.Run("CP-110_the_page_shows_the_teapot_image_the_heading_and_a_link_home", func(t *testing.T) {
		out := renderString(t, Teapot())
		img := regexp.MustCompile(`<img[^>]*>`).FindString(out)
		if got, _ := tagAttr(img, "src"); got != "/public/teapot.jpg" {
			t.Errorf("img src = %q", got)
		}
		if got, _ := tagAttr(img, "alt"); got != "Teapot" {
			t.Errorf("img alt = %q", got)
		}
		assertContains(t, out, `>418 I'm a teapot</h1>`)
		link := regexp.MustCompile(`(?s)<a [^>]*>\s*Brew another cup\s*</a>`).FindString(out)
		if href, _ := tagAttr(link, "href"); href != "/" {
			t.Errorf("link = %q, want href /", link)
		}
	})
}

func TestLogTeapotRequest(t *testing.T) {
	t.Run("CP-111_the_page_calls_the_APIs_teapot_route_once_on_load", func(t *testing.T) {
		out := renderString(t, Teapot())
		assertContains(t, out, "fetch(apiBaseUrl() + '/api/v1/teapot')", "response.status !== 418")
		if calls := regexp.MustCompile(`<script>__templ_logTeapotRequest_[0-9a-f]+\(\)</script>`).FindAllString(out, -1); len(calls) != 1 {
			t.Errorf("found %d call scripts, want 1 in:\n%s", len(calls), out)
		}
	})
}

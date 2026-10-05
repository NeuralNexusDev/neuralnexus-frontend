package button

import (
	"context"
	"html"
	"regexp"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func renderButton(t testing.TB, props []Props, children string) string {
	t.Helper()
	var out strings.Builder
	ctx := templ.WithChildren(context.Background(), templ.Raw(children))
	if err := Button(props...).Render(ctx, &out); err != nil {
		t.Fatalf("render: %v", err)
	}
	return out.String()
}

func openingTag(t testing.TB, out, element string) string {
	t.Helper()
	tag := regexp.MustCompile(`(?s)<` + element + `[\s>][^>]*>`).FindString(out)
	if tag == "" {
		t.Fatalf("no <%s in:\n%s", element, out)
	}
	return tag
}

func attrOf(tag, name string) (value string, ok bool) {
	tag = regexp.MustCompile(`class="[^"]*"`).ReplaceAllString(tag, "")
	found := regexp.MustCompile(`\s` + regexp.QuoteMeta(name) + `(?:="([^"]*)")?(?:\s|/?>)`).FindStringSubmatch(tag)
	if found == nil {
		return "", false
	}
	return html.UnescapeString(found[1]), true
}

func TestButton(t *testing.T) {
	t.Run("CP-112_no_props_renders_a_plain_button_with_its_children", func(t *testing.T) {
		out := renderButton(t, nil, "Go")
		tag := openingTag(t, out, "button")
		if got, _ := attrOf(tag, "type"); got != "button" {
			t.Errorf("type = %q, want button", got)
		}
		for _, name := range []string{"id", "disabled", "hx-get", "hx-post", "hx-put", "hx-delete", "hx-trigger", "hx-target", "hx-swap", "hx-replace-url"} {
			if _, ok := attrOf(tag, name); ok {
				t.Errorf("tag %s holds %s", tag, name)
			}
		}
		if n := strings.Count(out, "<button"); n != 1 || !strings.Contains(out, ">Go</button>") || strings.Contains(out, "<a ") {
			t.Errorf("output = %s, want one button holding Go and no link", out)
		}
	})

	t.Run("CP-113_an_empty_Type_becomes_button_and_a_set_Type_is_kept", func(t *testing.T) {
		cases := []struct {
			name string
			kind Type
			want string
		}{
			{"empty", "", "button"},
			{"button", TypeButton, "button"},
			{"submit", TypeSubmit, "submit"},
			{"reset", TypeReset, "reset"},
		}
		for _, tc := range cases {
			t.Run("CP-113_"+tc.name, func(t *testing.T) {
				tag := openingTag(t, renderButton(t, []Props{{Type: tc.kind}}, "Go"), "button")
				if got, _ := attrOf(tag, "type"); got != tc.want {
					t.Errorf("type = %q, want %q", got, tc.want)
				}
			})
		}
	})

	t.Run("CP-114_Disabled_disables_the_button", func(t *testing.T) {
		out := renderButton(t, []Props{{Disabled: true}}, "Go")
		if _, ok := attrOf(openingTag(t, out, "button"), "disabled"); !ok || strings.Count(out, "<button") != 1 {
			t.Errorf("output = %s, want one disabled button", out)
		}
	})

	t.Run("CP-115_a_set_Href_renders_a_link_with_the_id_the_address_and_the_children", func(t *testing.T) {
		out := renderButton(t, []Props{{ID: "go", Href: "/login"}}, "Go")
		tag := openingTag(t, out, "a")
		if got, _ := attrOf(tag, "id"); got != "go" {
			t.Errorf("id = %q, want go", got)
		}
		if got, _ := attrOf(tag, "href"); got != "/login" {
			t.Errorf("href = %q, want /login", got)
		}
		if n := strings.Count(out, "<a "); n != 1 || !strings.Contains(out, ">Go</a>") || strings.Contains(out, "<button") {
			t.Errorf("output = %s, want one link holding Go and no button", out)
		}
	})

	t.Run("CP-116_a_disabled_button_with_an_Href_renders_as_a_disabled_button_and_not_a_link", func(t *testing.T) {
		out := renderButton(t, []Props{{Href: "/login", Disabled: true}}, "Go")
		if _, ok := attrOf(openingTag(t, out, "button"), "disabled"); !ok {
			t.Errorf("the button is not disabled: %s", out)
		}
		if strings.Contains(out, "<a ") || strings.Contains(out, "href") {
			t.Errorf("output = %s, want no link and no href", out)
		}
	})

	t.Run("CP-117_Target_is_rendered_on_a_link_only_when_set", func(t *testing.T) {
		cases := []struct {
			name   string
			target string
			want   bool
		}{
			{"with_a_target", "_blank", true},
			{"without_a_target", "", false},
		}
		for _, tc := range cases {
			t.Run("CP-117_"+tc.name, func(t *testing.T) {
				tag := openingTag(t, renderButton(t, []Props{{Href: "/x", Target: tc.target}}, "Go"), "a")
				got, ok := attrOf(tag, "target")
				if ok != tc.want || (tc.want && got != "_blank") {
					t.Errorf("target = %q (present %t), want present %t; tag %s", got, ok, tc.want, tag)
				}
			})
		}
	})

	t.Run("CP-118_each_hx_prop_becomes_its_attribute_on_the_button", func(t *testing.T) {
		names := []string{"hx-get", "hx-post", "hx-put", "hx-delete", "hx-trigger", "hx-target", "hx-swap", "hx-replace-url"}
		t.Run("CP-118_all_set", func(t *testing.T) {
			tag := openingTag(t, renderButton(t, []Props{{
				HxGet: "/g", HxPost: "/p", HxPut: "/u", HxDelete: "/d", HxTrigger: "click", HxTarget: "#t", HxSwap: "outerHTML", HxReplaceUrl: "true",
			}}, "Go"), "button")
			for i, want := range []string{"/g", "/p", "/u", "/d", "click", "#t", "outerHTML", "true"} {
				if got, ok := attrOf(tag, names[i]); !ok || got != want {
					t.Errorf("%s = %q (present %t), want %q", names[i], got, ok, want)
				}
			}
		})
		t.Run("CP-118_none_set", func(t *testing.T) {
			tag := openingTag(t, renderButton(t, []Props{{}}, "Go"), "button")
			for _, name := range names {
				if _, ok := attrOf(tag, name); ok {
					t.Errorf("tag %s holds %s", tag, name)
				}
			}
		})
	})

	t.Run("CP-119_the_id_is_rendered_on_a_button_only_when_set", func(t *testing.T) {
		cases := []struct {
			name, id string
			want     bool
		}{
			{"with_an_id", "b1", true},
			{"without_an_id", "", false},
		}
		for _, tc := range cases {
			t.Run("CP-119_"+tc.name, func(t *testing.T) {
				tag := openingTag(t, renderButton(t, []Props{{ID: tc.id}}, "Go"), "button")
				got, ok := attrOf(tag, "id")
				if ok != tc.want || (tc.want && got != "b1") {
					t.Errorf("id = %q (present %t), want present %t; tag %s", got, ok, tc.want, tag)
				}
			})
		}
	})

	t.Run("CP-120_extra_attributes_are_rendered_on_both_the_button_and_the_link", func(t *testing.T) {
		cases := []struct{ name, href, element string }{
			{"button", "", "button"},
			{"link", "/x", "a"},
		}
		for _, tc := range cases {
			t.Run("CP-120_"+tc.name, func(t *testing.T) {
				out := renderButton(t, []Props{{Href: tc.href, Attributes: templ.Attributes{"data-x": "1"}}}, "Go")
				if got, ok := attrOf(openingTag(t, out, tc.element), "data-x"); !ok || got != "1" {
					t.Errorf("data-x = %q (present %t) in %s", got, ok, out)
				}
			})
		}
	})

	t.Run("CP-121_only_the_first_Props_value_is_used", func(t *testing.T) {
		out := renderButton(t, []Props{{ID: "one"}, {ID: "two"}}, "Go")
		if !strings.Contains(out, `id="one"`) || strings.Contains(out, `id="two"`) {
			t.Errorf("output = %s, want id one and no id two", out)
		}
	})
}

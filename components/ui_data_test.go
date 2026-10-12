package components

import (
	"regexp"
	"strings"
	"testing"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/test/testutil"
)

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

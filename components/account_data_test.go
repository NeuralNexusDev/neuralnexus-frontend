package components

import "testing"

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

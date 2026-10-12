package components

import (
	"os"
	"testing"
)

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

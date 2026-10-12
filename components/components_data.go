package components

import "os"

func steamOpenIDLoginURL() string {
	v := os.Getenv("STEAM_OPENID_LOGIN_URL")
	if v == "" {
		v = "https://steamcommunity.com/openid/login"
	}
	return v
}

// BeeSuggestionsData is the list of bee name suggestions waiting for review.
type BeeSuggestionsData struct {
	Names     []string
	FocusList bool
}

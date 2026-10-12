package components

import "testing"

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

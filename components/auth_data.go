package components

import "github.com/a-h/templ"

type oauthProvider struct {
	ID         string
	ElementID  string
	Label      string
	Background string
	Hover      string
	Icon       string
	Alt        string
	URL        templ.SafeURL
}

func oauthProviderFor(id string) (oauthProvider, bool) {
	for _, provider := range oauthProviders {
		if provider.ID == id {
			return provider, true
		}
	}
	return oauthProvider{}, false
}

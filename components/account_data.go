package components

// AccountPlatform is a sign-in platform an account can link.
type AccountPlatform struct {
	ID   string
	Name string
}

// AccountData is what the account page shows.
type AccountData struct {
	Username      string
	PasswordAuth  bool
	SettingsError string
	LinksError    string
	Links         map[string]LinkedAccount
	AdminLink     bool
}

// AccountSettingsData is the password login setting.
type AccountSettingsData struct {
	PasswordAuth bool
	Error        string
}

// HasLink reports whether the account is linked to the platform.
func (d AccountData) HasLink(platform string) bool {
	_, ok := d.Links[platform]
	return ok
}

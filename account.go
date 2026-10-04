package main

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
)

func accountPlatform(id string) (components.AccountPlatform, bool) {
	index := slices.IndexFunc(components.AccountPlatforms, func(p components.AccountPlatform) bool { return p.ID == id })
	if index < 0 {
		return components.AccountPlatform{}, false
	}
	return components.AccountPlatforms[index], true
}

func loadAccountSettings(a adminAPI) (components.AccountSettingsData, error) {
	settings, err := adminGet[struct {
		PasswordAuth bool `json:"password_auth"`
	}](a, "/users/me/settings", "Failed to load account settings")
	return components.AccountSettingsData{PasswordAuth: settings.PasswordAuth}, err
}

func loadAccountLinks(a adminAPI) (map[string]components.LinkedAccount, error) {
	links, err := adminGet[[]components.LinkedAccount](a, "/users/me/links", "Failed to load linked accounts")
	byPlatform := make(map[string]components.LinkedAccount, len(links))
	for _, link := range links {
		byPlatform[link.Platform] = link
	}
	return byPlatform, err
}

func accountContentHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	account, err := adminGet[struct {
		Username string `json:"username"`
	}](a, "/users/me", "Failed to load your account")
	if err != nil {
		failFragment(w, r, err)
		return
	}
	data := components.AccountData{Username: account.Username}
	settings, err := loadAccountSettings(a)
	if data.SettingsError, err = secondary(err); err != nil {
		failFragment(w, r, err)
		return
	}
	data.PasswordAuth = settings.PasswordAuth
	if data.Links, err = loadAccountLinks(a); err != nil {
		if data.LinksError, err = secondary(err); err != nil {
			failFragment(w, r, err)
			return
		}
	}
	renderAll(w, r, components.AccountContent(data))
}

// failSettings answers a failed change to the password login with the error, and with the setting as it stands so the checkbox goes back.
func failSettings(w http.ResponseWriter, r *http.Request, a adminAPI, err error) {
	current, loadErr := loadAccountSettings(a)
	if loadErr != nil {
		failFragment(w, r, err)
		return
	}
	failFragment(w, r, err, components.AccountPasswordLogin(current, true))
}

func accountSettingsHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	body := map[string]bool{"password_auth": r.PostForm.Get("password_auth") == "true"}
	if _, err := adminSend[struct{}](a, http.MethodPatch, "/users/me/settings", body, "Failed to update account settings"); err != nil {
		failSettings(w, r, a, err)
		return
	}
	data, err := loadAccountSettings(a)
	if err != nil {
		failFragment(w, r, afterWrite(err))
		return
	}
	renderAll(w, r, components.AccountPasswordLogin(data, false))
}

// accountLinkRow reloads the account's links and answers with the row of one platform.
func accountLinkRow(w http.ResponseWriter, r *http.Request, a adminAPI, platform components.AccountPlatform) {
	links, err := loadAccountLinks(a)
	if err != nil {
		failFragment(w, r, afterWrite(err))
		return
	}
	link, linked := links[platform.ID]
	renderAll(w, r, components.AccountLinkRow(platform, link, linked, false))
}

// failLink answers a failed change to a platform with the error, and with the row as it stands so the checkbox goes back.
func failLink(w http.ResponseWriter, r *http.Request, a adminAPI, platform components.AccountPlatform, err error) {
	links, loadErr := loadAccountLinks(a)
	if loadErr != nil {
		failFragment(w, r, err)
		return
	}
	link, linked := links[platform.ID]
	failFragment(w, r, err, components.AccountLinkRow(platform, link, linked, true))
}

func accountLinkHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	platform, ok := accountPlatform(r.PathValue("platform"))
	if !ok {
		failFragment(w, r, &adminError{Status: http.StatusNotFound, Message: "Unknown platform"})
		return
	}
	body := map[string]bool{"login_enabled": r.PostForm.Get("login_enabled") == "true"}
	if _, err := adminSend[struct{}](a, http.MethodPatch, "/users/me/link/"+url.PathEscape(platform.ID), body, "Failed to update platform"); err != nil {
		failLink(w, r, a, platform, err)
		return
	}
	accountLinkRow(w, r, a, platform)
}

func accountUnlinkHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	platform, ok := accountPlatform(r.PathValue("platform"))
	if !ok {
		failFragment(w, r, &adminError{Status: http.StatusNotFound, Message: "Unknown platform"})
		return
	}
	if _, err := adminSend[struct{}](a, http.MethodDelete, "/users/me/link/"+url.PathEscape(platform.ID), nil, "Failed to unlink platform"); err != nil {
		failLink(w, r, a, platform, err)
		return
	}
	accountLinkRow(w, r, a, platform)
}

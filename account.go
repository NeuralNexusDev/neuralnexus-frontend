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

func loadAccountSettings(a apiSession) (components.AccountSettingsData, error) {
	settings, err := apiGet[struct {
		PasswordAuth bool `json:"password_auth"`
	}](a, "/users/me/settings", "Failed to load account settings")
	return components.AccountSettingsData{PasswordAuth: settings.PasswordAuth}, err
}

func loadAccountLinks(a apiSession) (map[string]components.LinkedAccount, error) {
	links, err := apiGet[[]components.LinkedAccount](a, "/users/me/links", "Failed to load linked accounts")
	byPlatform := make(map[string]components.LinkedAccount, len(links))
	for _, link := range links {
		byPlatform[link.Platform] = link
	}
	return byPlatform, err
}

func accountContentHandler(w http.ResponseWriter, r *http.Request, a apiSession) {
	account, err := apiGet[struct {
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
	permissions, _ := apiGet[[]string](a, "/users/me/permissions", loadYourPermsFailed)
	data.AdminLink = hasPermission(permissions, "users.admin") || hasPermission(permissions, "roles.admin")
	renderAll(w, r, components.AccountContent(data))
}

func accountSettingsHandler(w http.ResponseWriter, r *http.Request, a apiSession) {
	enabled := r.Form.Get("password_auth") == "true"
	body := map[string]bool{"password_auth": enabled}
	if _, err := apiSend[struct{}](a, http.MethodPatch, "/users/me/settings", body, "Failed to update account settings"); err != nil {
		failFragment(w, r, err, components.AccountPasswordLogin(components.AccountSettingsData{PasswordAuth: !enabled}, true))
		return
	}
	data, err := loadAccountSettings(a)
	if err != nil {
		failFragment(w, r, afterWrite(err))
		return
	}
	renderAll(w, r, components.AccountPasswordLogin(data, false))
}

func accountLinkRow(w http.ResponseWriter, r *http.Request, a apiSession, platform components.AccountPlatform) {
	links, err := loadAccountLinks(a)
	if err != nil {
		failFragment(w, r, afterWrite(err))
		return
	}
	link, linked := links[platform.ID]
	renderAll(w, r, components.AccountLinkRow(platform, link, linked, false))
}

func accountLinkHandler(w http.ResponseWriter, r *http.Request, a apiSession) {
	platform, ok := accountPlatform(r.PathValue("platform"))
	if !ok {
		failFragment(w, r, &apiError{Status: http.StatusNotFound, Message: unknownPlatform})
		return
	}
	enabled := r.Form.Get("login_enabled") == "true"
	body := map[string]bool{"login_enabled": enabled}
	if _, err := apiSend[struct{}](a, http.MethodPatch, "/users/me/link/"+url.PathEscape(platform.ID), body, "Failed to update platform"); err != nil {
		failFragment(w, r, err, components.AccountLoginInput(platform, !enabled, false, true))
		return
	}
	accountLinkRow(w, r, a, platform)
}

func accountUnlinkHandler(w http.ResponseWriter, r *http.Request, a apiSession) {
	platform, ok := accountPlatform(r.PathValue("platform"))
	if !ok {
		failFragment(w, r, &apiError{Status: http.StatusNotFound, Message: unknownPlatform})
		return
	}
	if _, err := apiSend[struct{}](a, http.MethodDelete, "/users/me/link/"+url.PathEscape(platform.ID), nil, "Failed to unlink platform"); err != nil {
		failFragment(w, r, err)
		return
	}
	accountLinkRow(w, r, a, platform)
}

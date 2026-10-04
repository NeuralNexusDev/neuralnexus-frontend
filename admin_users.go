package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
)

const (
	usersPageSize = 200
	searchPages   = 5
)

func sameSet(a []string, b []string) bool {
	held := make(map[string]bool, len(b))
	for _, id := range b {
		held[id] = true
	}
	if len(a) != len(held) {
		return false
	}
	for _, id := range a {
		if !held[id] {
			return false
		}
	}
	return true
}

func listRoles(a apiSession) ([]components.Role, bool, error) {
	roles, err := apiGet[[]components.Role](a, "/roles", loadRolesFailed)
	var failure *apiError
	if errors.As(err, &failure) && failure.Status == http.StatusForbidden {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return roles, true, nil
}

// The API has no search, so a search reads pages of users and matches them here.
func loadUsersPage(a apiSession, offset int, search string) (components.AdminUsersData, error) {
	search = strings.TrimSpace(search)
	needle := strings.ToLower(search)
	pages := 1
	if needle != "" {
		pages = searchPages
	}
	var users []components.UserAccount
	next := offset
	more := false
	for range pages {
		page, err := apiGet[[]components.UserAccount](a, fmt.Sprintf("/users?limit=%d&offset=%d", usersPageSize, next), "Failed to load users")
		if err != nil {
			return components.AdminUsersData{}, err
		}
		next += len(page)
		more = len(page) == usersPageSize
		for _, user := range page {
			if strings.Contains(strings.ToLower(user.Username), needle) || strings.Contains(user.UserID, needle) {
				users = append(users, user)
			}
		}
		if !more {
			break
		}
	}
	roles, readable, err := listRoles(a)
	if err != nil {
		return components.AdminUsersData{}, err
	}
	var names map[string]string
	if readable {
		names = make(map[string]string, len(roles))
		for _, role := range roles {
			names[role.ID] = role.Name
		}
	}
	return components.AdminUsersData{
		Users:      users,
		RoleNames:  names,
		Search:     search,
		Start:      offset,
		NextOffset: next,
		More:       more,
	}, nil
}

func adminUsersListHandler(w http.ResponseWriter, r *http.Request, a apiSession) {
	data, err := loadUsersPage(a, 0, "")
	if err != nil {
		failFragment(w, r, err)
		return
	}
	renderAll(w, r, components.AdminUsersList(data))
}

func adminUserRowsHandler(w http.ResponseWriter, r *http.Request, a apiSession) {
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		var err error
		if offset, err = strconv.Atoi(raw); err != nil || offset < 0 {
			failFragment(w, r, invalidInput("The offset must be a whole number from 0"))
			return
		}
	}
	data, err := loadUsersPage(a, offset, r.URL.Query().Get("search"))
	if err != nil {
		failFragment(w, r, err)
		return
	}
	data.FocusFirst = offset > 0
	renderAll(w, r, components.AdminUserRows(data), components.AdminUsersCount(data, true))
}

func loadUserEditor(a apiSession, id string, user *components.UserAccount, withLinks bool) (components.AdminUserData, error) {
	path := "/users/" + url.PathEscape(id)
	if user == nil {
		loaded, err := apiGet[components.UserAccount](a, path, "Failed to load the user")
		if err != nil {
			return components.AdminUserData{}, err
		}
		user = &loaded
	}
	roles, readable, err := listRoles(a)
	if err != nil {
		return components.AdminUserData{}, err
	}
	data := components.AdminUserData{User: *user, Roles: roles, RolesReadable: readable}
	if withLinks {
		links, err := apiGet[[]components.LinkedAccount](a, path+"/links", "Failed to load the linked accounts")
		if data.LinksError, err = secondary(err); err != nil {
			return components.AdminUserData{}, err
		}
		data.Links = links
	}
	permissions, err := apiGet[[]string](a, path+"/permissions", "Failed to load the permissions")
	if data.PermissionsErr, err = secondary(err); err != nil {
		return components.AdminUserData{}, err
	}
	data.Permissions = permissions
	return data, nil
}

func adminUserEditorHandler(w http.ResponseWriter, r *http.Request, a apiSession) {
	data, err := loadUserEditor(a, r.PathValue("id"), nil, true)
	if err != nil {
		failFragment(w, r, err)
		return
	}
	renderAll(w, r, components.AdminUserContent(data))
}

func renderUserSave(w http.ResponseWriter, r *http.Request, data components.AdminUserData, status string) {
	renderAll(w, r,
		components.AdminUserForm(data),
		components.AdminUserHeader(data, true),
		components.AdminUserPermissions(data, true),
		components.StatusLine(components.AdminUserStatusID, status),
	)
}

func adminUserSaveHandler(w http.ResponseWriter, r *http.Request, a apiSession) {
	id := r.PathValue("id")
	body := map[string]any{}
	if raw := r.Form.Get("username"); raw != r.Form.Get("loaded_username") {
		username := strings.TrimSpace(raw)
		if username == "" {
			failEditor(w, r, components.AdminUserStatusID, invalidInput("Enter a username"), components.AdminUserUsername(raw, "Enter a username", true))
			return
		}
		body["username"] = username
	}
	if r.Form.Get("roles_editable") == "1" {
		roles := append(append([]string{}, r.Form["roles"]...), r.Form["kept_roles"]...)
		if !sameSet(roles, r.Form["loaded_roles"]) {
			body["roles"] = roles
		}
	}
	if len(body) == 0 {
		nothingToSave(w, r, components.AdminUserStatusID)
		return
	}
	saved, err := apiSend[components.UserAccount](a, http.MethodPut, "/users/"+url.PathEscape(id), body, "Failed to save the user")
	if err != nil {
		var restore []templ.Component
		if message := fieldRefusal(err); message != "" && body["username"] != nil {
			restore = append(restore, components.AdminUserUsername(r.Form.Get("username"), message, true))
		}
		failEditor(w, r, components.AdminUserStatusID, err, restore...)
		return
	}
	data, err := loadUserEditor(a, id, &saved, false)
	if err != nil {
		written := components.AdminUserData{User: saved}
		failEditor(w, r, components.AdminUserStatusID, afterWrite(err), components.AdminUserHeader(written, true), components.AdminUserLoaded(written, true))
		return
	}
	status := "Saved"
	if data.PermissionsErr != "" {
		status = "Saved, but the permissions below are out of date"
	}
	renderUserSave(w, r, data, status)
}

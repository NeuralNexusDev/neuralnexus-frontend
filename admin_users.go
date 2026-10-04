package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
)

const (
	usersPageSize = 200
	// searchPages is how many pages of users one search request reads.
	searchPages = 5
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

// listRoles returns every role, and false when the caller may not read roles.
func listRoles(a adminAPI) ([]components.Role, bool, error) {
	roles, err := adminGet[[]components.Role](a, "/roles", "Failed to load roles")
	var failure *adminError
	if errors.As(err, &failure) && failure.Status == http.StatusForbidden {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return roles, true, nil
}

// loadUsersPage reads the users from the offset, and with a search reads on until it has covered searchPages pages.
// The API cannot search, so the matching happens here.
func loadUsersPage(a adminAPI, offset int, search string) (components.AdminUsersData, error) {
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
		page, err := adminGet[[]components.UserAccount](a, fmt.Sprintf("/users?limit=%d&offset=%d", usersPageSize, next), "Failed to load users")
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

func adminUsersListHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	data, err := loadUsersPage(a, 0, "")
	if err != nil {
		failFragment(w, r, err)
		return
	}
	renderAll(w, r, components.AdminUsersList(data))
}

func adminUserRowsHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
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
	renderAll(w, r, components.AdminUserRows(data))
}

// loadUserEditor loads the user editor's data, with the linked accounts only when the page needs them.
func loadUserEditor(a adminAPI, id string, user *components.UserAccount, withLinks bool) (components.AdminUserData, error) {
	path := "/users/" + url.PathEscape(id)
	if user == nil {
		loaded, err := adminGet[components.UserAccount](a, path, "Failed to load the user")
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
		links, err := adminGet[[]components.LinkedAccount](a, path+"/links", "Failed to load the linked accounts")
		if data.LinksError, err = secondary(err); err != nil {
			return components.AdminUserData{}, err
		}
		data.Links = links
	}
	permissions, err := adminGet[[]string](a, path+"/permissions", "Failed to load the permissions")
	if data.PermissionsErr, err = secondary(err); err != nil {
		return components.AdminUserData{}, err
	}
	data.Permissions = permissions
	return data, nil
}

func adminUserEditorHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	data, err := loadUserEditor(a, r.PathValue("id"), nil, true)
	if err != nil {
		failFragment(w, r, err)
		return
	}
	renderAll(w, r, components.AdminUserContent(data))
}

// renderUserSave answers a save with the form, and with the header and permissions, which the save can change.
func renderUserSave(w http.ResponseWriter, r *http.Request, data components.AdminUserData, status string) {
	renderAll(w, r,
		components.AdminUserForm(data),
		components.AdminUserHeader(data, true),
		components.AdminUserPermissions(data, true),
		components.AdminStatus("admin-user-status", status),
	)
}

func adminUserSaveHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	id := r.PathValue("id")
	body := map[string]any{}
	if raw := r.PostForm.Get("username"); raw != r.PostForm.Get("loaded_username") {
		username := strings.TrimSpace(raw)
		if username == "" {
			failFragment(w, r, invalidInput("Enter a username"))
			return
		}
		body["username"] = username
	}
	if r.PostForm.Get("roles_editable") == "1" {
		roles := append(append([]string{}, r.PostForm["roles"]...), r.PostForm["kept_roles"]...)
		if !sameSet(roles, r.PostForm["loaded_roles"]) {
			body["roles"] = roles
		}
	}
	if len(body) == 0 {
		data, err := loadUserEditor(a, id, nil, false)
		if err != nil {
			failFragment(w, r, err)
			return
		}
		renderUserSave(w, r, data, "Nothing to save")
		return
	}
	saved, err := adminSend[components.UserAccount](a, http.MethodPut, "/users/"+url.PathEscape(id), body, "Failed to save the user")
	if err != nil {
		failFragment(w, r, err)
		return
	}
	data, err := loadUserEditor(a, id, &saved, false)
	if err != nil {
		failFragment(w, r, afterWrite(err))
		return
	}
	status := "Saved"
	if data.PermissionsErr != "" {
		status = "Saved, but the permissions below are out of date"
	}
	renderUserSave(w, r, data, status)
}

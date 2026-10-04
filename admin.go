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
	// searchPages is how many pages of users one search request reads.
	searchPages = 5
	maxGrantInt = 1 << 53
)

func noStoreHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", noStore)
		next.ServeHTTP(w, r)
	})
}

// adminRoute serves a page, calling the API as the signed-in user.
func adminRoute(handler func(http.ResponseWriter, *http.Request, adminAPI)) http.Handler {
	return noStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, adminAPI{r: r})
	}))
}

// adminAction serves a change made by htmx. The HX-Request header is one a cross-site form cannot send.
func adminAction(handler func(http.ResponseWriter, *http.Request, adminAPI)) http.Handler {
	return adminRoute(func(w http.ResponseWriter, r *http.Request, a adminAPI) {
		if r.Header.Get("HX-Request") != "true" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		handler(w, r, a)
	})
}

func errorStatus(err error) (int, string) {
	var failure *adminError
	if errors.As(err, &failure) {
		return failure.Status, failure.Message
	}
	return http.StatusInternalServerError, "Something went wrong"
}

// failPage answers a page request with the page showing the error, or sends a signed-out visitor to the login page.
func failPage(w http.ResponseWriter, r *http.Request, err error, page func(message string) templ.Component) {
	if errors.Is(err, errUnauthorized) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	status, message := errorStatus(err)
	templ.Handler(page(message), templ.WithStatus(status)).ServeHTTP(w, r)
}

// failFragment answers an htmx request with the error for the banner, or sends a signed-out visitor to the login page.
func failFragment(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errUnauthorized) {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	status, message := errorStatus(err)
	w.Header().Set("HX-Retarget", "#admin-error")
	w.Header().Set("HX-Reswap", "innerHTML")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	parts := []templ.Component{components.AdminErrorText(message)}
	if id := statusLineID(r.URL.Path); id != "" {
		parts = append(parts, components.AdminStatus(id, ""))
	}
	render(w, r, parts...)
}

// statusLineID returns the status line of the editor a path belongs to, which a failed change empties.
func statusLineID(path string) string {
	switch {
	case path == "/admin/users/rows":
		return ""
	case strings.HasPrefix(path, "/admin/users/"):
		return "admin-user-status"
	case strings.HasPrefix(path, "/admin/roles/"):
		return "admin-role-status"
	}
	return ""
}

func render(w http.ResponseWriter, r *http.Request, parts ...templ.Component) {
	for _, part := range parts {
		if err := part.Render(r.Context(), w); err != nil {
			return
		}
	}
}

// renderAll answers an htmx request with the fragments and an empty error banner, since the change worked.
func renderAll(w http.ResponseWriter, r *http.Request, parts ...templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, append(parts, components.AdminErrorClear())...)
}

func redirectHTMX(w http.ResponseWriter, location string) {
	w.Header().Set("HX-Redirect", location)
	w.WriteHeader(http.StatusOK)
}

func hasPermission(permissions []string, node string) bool {
	for _, permission := range permissions {
		if permission == node || strings.HasPrefix(permission, node+":") {
			return true
		}
	}
	return false
}

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

// secondary turns a failed lookup the page can do without into the message to show beside it, and keeps a sign-out an error.
func secondary(err error) (string, error) {
	if err == nil {
		return "", nil
	}
	if errors.Is(err, errUnauthorized) {
		return "", err
	}
	_, message := errorStatus(err)
	return message, nil
}

// accountAdminLinkHandler answers the account page with the link to the dashboard, or with nothing for an account without admin permissions.
// The link is a convenience, so a failed lookup shows no link.
func accountAdminLinkHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	permissions, err := adminGet[[]string](a, "/users/me/permissions", "Failed to load your permissions")
	if err != nil || !(hasPermission(permissions, "users.admin") || hasPermission(permissions, "roles.admin")) {
		return
	}
	templ.Handler(components.AdminDashboardLink()).ServeHTTP(w, r)
}

func adminDashboardHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	permissions, err := adminGet[[]string](a, "/users/me/permissions", "Failed to load your permissions")
	if err != nil {
		failPage(w, r, err, func(message string) templ.Component {
			return components.AdminDashboardPage(components.AdminDashboardData{Error: message})
		})
		return
	}
	templ.Handler(components.AdminDashboardPage(components.AdminDashboardData{
		Users: hasPermission(permissions, "users.admin"),
		Roles: hasPermission(permissions, "roles.admin"),
	})).ServeHTTP(w, r)
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
		Loaded:     true,
	}, nil
}

func adminUsersHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	data, err := loadUsersPage(a, 0, "")
	if err != nil {
		failPage(w, r, err, func(message string) templ.Component {
			return components.AdminUsersPage(components.AdminUsersData{Error: message})
		})
		return
	}
	templ.Handler(components.AdminUsersPage(data)).ServeHTTP(w, r)
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

func loadUserEditor(a adminAPI, id string, user *components.UserAccount) (components.AdminUserData, error) {
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
	data := components.AdminUserData{User: *user, Roles: roles, RolesReadable: readable, Loaded: true}
	links, err := adminGet[[]components.LinkedAccount](a, path+"/links", "Failed to load the linked accounts")
	if data.LinksError, err = secondary(err); err != nil {
		return components.AdminUserData{}, err
	}
	data.Links = links
	permissions, err := adminGet[[]string](a, path+"/permissions", "Failed to load the permissions")
	if data.PermissionsErr, err = secondary(err); err != nil {
		return components.AdminUserData{}, err
	}
	data.Permissions = permissions
	return data, nil
}

func adminUserHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	data, err := loadUserEditor(a, r.PathValue("id"), nil)
	if err != nil {
		failPage(w, r, err, func(message string) templ.Component {
			return components.AdminUserPage(components.AdminUserData{Error: message})
		})
		return
	}
	templ.Handler(components.AdminUserPage(data)).ServeHTTP(w, r)
}

// afterWrite words a failure to reload what was just changed so it is not mistaken for a failure to change it.
func afterWrite(err error) error {
	if err == nil || errors.Is(err, errUnauthorized) {
		return err
	}
	status, message := errorStatus(err)
	return &adminError{Status: status, Message: "The change was made, but the page could not be refreshed: " + message}
}

func adminUserSaveHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		failFragment(w, r, invalidInput("The form could not be read"))
		return
	}
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
		data, err := loadUserEditor(a, id, nil)
		if err != nil {
			failFragment(w, r, err)
			return
		}
		renderAll(w, r, components.AdminUserEditor(data), components.AdminStatus("admin-user-status", "Nothing to save"))
		return
	}
	saved, err := adminSend[components.UserAccount](a, http.MethodPut, "/users/"+url.PathEscape(id), body, "Failed to save the user")
	if err != nil {
		failFragment(w, r, err)
		return
	}
	data, err := loadUserEditor(a, id, &saved)
	if err != nil {
		failFragment(w, r, afterWrite(err))
		return
	}
	status := "Saved"
	if data.PermissionsErr != "" {
		status = "Saved, but the permissions below are out of date"
	}
	renderAll(w, r, components.AdminUserEditor(data), components.AdminStatus("admin-user-status", status))
}

func adminRolesHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	roles, err := adminGet[[]components.Role](a, "/roles", "Failed to load roles")
	if err != nil {
		failPage(w, r, err, func(message string) templ.Component {
			return components.AdminRolesPage(components.AdminRolesData{Error: message})
		})
		return
	}
	templ.Handler(components.AdminRolesPage(components.AdminRolesData{Roles: roles, Loaded: true})).ServeHTTP(w, r)
}

func adminRoleCreateHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	if err := r.ParseForm(); err != nil {
		failFragment(w, r, invalidInput("The form could not be read"))
		return
	}
	body := map[string]string{
		"name":        strings.TrimSpace(r.PostForm.Get("name")),
		"description": strings.TrimSpace(r.PostForm.Get("description")),
	}
	role, err := adminSend[components.Role](a, http.MethodPost, "/roles", body, "Failed to create the role")
	if err != nil {
		failFragment(w, r, err)
		return
	}
	redirectHTMX(w, "/admin/roles/"+url.PathEscape(role.ID))
}

func loadRole(a adminAPI, id string) (components.AdminRoleData, error) {
	role, err := adminGet[components.Role](a, "/roles/"+url.PathEscape(id), "Failed to load the role")
	if err != nil {
		return components.AdminRoleData{}, err
	}
	return components.AdminRoleData{Role: role, Loaded: true}, nil
}

func loadRoleEditor(a adminAPI, id string) (components.AdminRoleData, error) {
	data, err := loadRole(a, id)
	if err != nil {
		return components.AdminRoleData{}, err
	}
	data.Catalogue, err = adminGet[[]components.Permission](a, "/permissions", "Failed to load permissions")
	if err != nil {
		return components.AdminRoleData{}, err
	}
	return data, nil
}

func adminRoleHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	data, err := loadRoleEditor(a, r.PathValue("id"))
	if err != nil {
		failPage(w, r, err, func(message string) templ.Component {
			return components.AdminRolePage(components.AdminRoleData{Error: message})
		})
		return
	}
	templ.Handler(components.AdminRolePage(data)).ServeHTTP(w, r)
}

func adminRoleSaveHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		failFragment(w, r, invalidInput("The form could not be read"))
		return
	}
	body := map[string]string{}
	if raw := r.PostForm.Get("name"); raw != r.PostForm.Get("loaded_name") {
		name := strings.TrimSpace(raw)
		if name == "" {
			failFragment(w, r, invalidInput("Enter a name"))
			return
		}
		body["name"] = name
	}
	if raw := r.PostForm.Get("description"); raw != r.PostForm.Get("loaded_description") {
		body["description"] = strings.TrimSpace(raw)
	}
	status := "Nothing to save"
	if len(body) > 0 {
		if _, err := adminSend[components.Role](a, http.MethodPatch, "/roles/"+url.PathEscape(id), body, "Failed to save the role"); err != nil {
			failFragment(w, r, err)
			return
		}
		status = "Saved"
	}
	data, err := loadRole(a, id)
	if err != nil {
		failFragment(w, r, afterWrite(err))
		return
	}
	renderAll(w, r,
		components.AdminRoleForm(data),
		components.AdminRoleHeader(data, true),
		components.AdminRoleDelete(data, true),
		components.AdminStatus("admin-role-status", status),
	)
}

func adminRoleDeleteHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	if _, err := adminSend[struct{}](a, http.MethodDelete, "/roles/"+url.PathEscape(r.PathValue("id")), nil, "Failed to delete the role"); err != nil {
		failFragment(w, r, err)
		return
	}
	redirectHTMX(w, "/admin/roles")
}

// grantBody returns the request body that grants the permission with the typed value, and nil for a permission granted as is.
func grantBody(permission components.Permission, raw string) (any, error) {
	text := strings.TrimSpace(raw)
	switch permission.ValueType {
	case "":
		return nil, nil
	case "int":
		number, err := strconv.ParseInt(text, 10, 64)
		if err != nil || strconv.FormatInt(number, 10) != text || number > maxGrantInt || number < -maxGrantInt {
			return nil, invalidInput(fmt.Sprintf("Enter a whole number from -%d to %d", int64(maxGrantInt), int64(maxGrantInt)))
		}
		return map[string]any{"value": number}, nil
	case "string_list":
		var items []string
		for _, line := range strings.Split(raw, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				items = append(items, line)
			}
		}
		if len(items) == 0 {
			return nil, invalidInput("Enter at least one item")
		}
		return map[string]any{"value": items}, nil
	case "string":
		if text == "" {
			return nil, invalidInput("Enter a value")
		}
		return map[string]any{"value": text}, nil
	default:
		return nil, invalidInput("This permission has a value type that cannot be edited here")
	}
}

func findPermission(a adminAPI, id string) (components.Permission, error) {
	catalogue, err := adminGet[[]components.Permission](a, "/permissions", "Failed to load permissions")
	if err != nil {
		return components.Permission{}, err
	}
	for _, permission := range catalogue {
		if permission.ID == id {
			return permission, nil
		}
	}
	return components.Permission{}, &adminError{Status: http.StatusNotFound, Message: "Permission not found"}
}

// roleDrafts returns what was typed in each granted permission's value field, apart from the one the change replaces.
func roleDrafts(form url.Values, except string) map[string]string {
	drafts := map[string]string{}
	for key, values := range form {
		id, ok := strings.CutPrefix(key, "value_")
		if ok && id != except && len(values) > 0 {
			drafts[id] = values[0]
		}
	}
	return drafts
}

// renderGrants reloads the role and answers with its granted permissions, which keep the drafts typed in them.
func renderGrants(w http.ResponseWriter, r *http.Request, a adminAPI, drafts map[string]string, grantPermission string, grantValue string, focusList bool) {
	data, err := loadRoleEditor(a, r.PathValue("id"))
	if err != nil {
		failFragment(w, r, afterWrite(err))
		return
	}
	data.Drafts = drafts
	data.GrantPermission = grantPermission
	data.GrantValue = grantValue
	data.FocusList = focusList || len(data.Available()) == 0
	renderAll(w, r, components.AdminRoleGrants(data), components.AdminStatus("admin-role-status", ""))
}

// putGrant grants the permission to the role with the value typed for it.
func putGrant(w http.ResponseWriter, r *http.Request, a adminAPI, permissionID string, raw string) bool {
	permission, err := findPermission(a, permissionID)
	if err != nil {
		failFragment(w, r, err)
		return false
	}
	body, err := grantBody(permission, raw)
	if err != nil {
		failFragment(w, r, err)
		return false
	}
	path := "/roles/" + url.PathEscape(r.PathValue("id")) + "/permissions/" + url.PathEscape(permissionID)
	if _, err := adminSend[struct{}](a, http.MethodPut, path, body, "Failed to grant the permission"); err != nil {
		failFragment(w, r, err)
		return false
	}
	return true
}

func adminRoleGrantHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	if err := r.ParseForm(); err != nil {
		failFragment(w, r, invalidInput("The form could not be read"))
		return
	}
	if putGrant(w, r, a, r.PostForm.Get("grant_permission"), r.PostForm.Get("grant_value")) {
		renderGrants(w, r, a, roleDrafts(r.PostForm, ""), "", "", false)
	}
}

func adminRoleValueHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	if err := r.ParseForm(); err != nil {
		failFragment(w, r, invalidInput("The form could not be read"))
		return
	}
	permissionID := r.PathValue("permission")
	if putGrant(w, r, a, permissionID, r.PostForm.Get("value_"+permissionID)) {
		renderGrants(w, r, a, roleDrafts(r.PostForm, permissionID), r.PostForm.Get("grant_permission"), r.PostForm.Get("grant_value"), true)
	}
}

func adminRoleRemoveHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	if err := r.ParseForm(); err != nil {
		failFragment(w, r, invalidInput("The form could not be read"))
		return
	}
	permissionID := r.PathValue("permission")
	path := "/roles/" + url.PathEscape(r.PathValue("id")) + "/permissions/" + url.PathEscape(permissionID)
	if _, err := adminSend[struct{}](a, http.MethodDelete, path, nil, "Failed to remove the permission"); err != nil {
		failFragment(w, r, err)
		return
	}
	renderGrants(w, r, a, roleDrafts(r.PostForm, permissionID), r.PostForm.Get("grant_permission"), r.PostForm.Get("grant_value"), true)
}

func adminRoleGrantValueHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	permission, err := findPermission(a, r.URL.Query().Get("grant_permission"))
	if err != nil {
		failFragment(w, r, err)
		return
	}
	renderAll(w, r, components.AdminGrantValue(permission, ""))
}

func adminPermissionsHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	permissions, err := adminGet[[]components.Permission](a, "/permissions", "Failed to load permissions")
	if err != nil {
		failPage(w, r, err, func(message string) templ.Component {
			return components.AdminPermissionsPage(components.AdminPermissionsData{Error: message})
		})
		return
	}
	templ.Handler(components.AdminPermissionsPage(components.AdminPermissionsData{Permissions: permissions, Loaded: true})).ServeHTTP(w, r)
}

func loadPermissions(a adminAPI, focusList bool) (components.AdminPermissionsData, error) {
	permissions, err := adminGet[[]components.Permission](a, "/permissions", "Failed to load permissions")
	if err != nil {
		return components.AdminPermissionsData{}, err
	}
	return components.AdminPermissionsData{Permissions: permissions, FocusList: focusList, Loaded: true}, nil
}

func adminPermissionCreateHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	if err := r.ParseForm(); err != nil {
		failFragment(w, r, invalidInput("The form could not be read"))
		return
	}
	body := map[string]string{
		"node":        strings.TrimSpace(r.PostForm.Get("node")),
		"description": strings.TrimSpace(r.PostForm.Get("description")),
	}
	valueType := r.PostForm.Get("value_type")
	if valueType != "" {
		body["value_type"] = valueType
	}
	if valueType == "int" {
		body["merge"] = r.PostForm.Get("merge")
	}
	if _, err := adminSend[components.Permission](a, http.MethodPost, "/permissions", body, "Failed to create the permission"); err != nil {
		failFragment(w, r, err)
		return
	}
	data, err := loadPermissions(a, false)
	if err != nil {
		failFragment(w, r, afterWrite(err))
		return
	}
	renderAll(w, r, components.AdminPermissionList(data), components.AdminPermissionForm(true))
}

func adminPermissionDeleteHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	if _, err := adminSend[struct{}](a, http.MethodDelete, "/permissions/"+url.PathEscape(r.PathValue("id")), nil, "Failed to delete the permission"); err != nil {
		failFragment(w, r, err)
		return
	}
	data, err := loadPermissions(a, true)
	if err != nil {
		failFragment(w, r, afterWrite(err))
		return
	}
	renderAll(w, r, components.AdminPermissionList(data))
}

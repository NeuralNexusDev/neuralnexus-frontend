package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
)

// maxGrantInt is the largest whole number the API accepts as a granted value, in either direction.
const maxGrantInt = 1 << 53

func adminRolesListHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	roles, err := adminGet[[]components.Role](a, "/roles", "Failed to load roles")
	if err != nil {
		failFragment(w, r, err)
		return
	}
	renderAll(w, r, components.AdminRolesContent(components.AdminRolesData{Roles: roles}))
}

func adminRoleCreateHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	body := map[string]string{
		"name":        strings.TrimSpace(r.Form.Get("name")),
		"description": strings.TrimSpace(r.Form.Get("description")),
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
	return components.AdminRoleData{Role: role}, nil
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

func adminRoleEditorHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	data, err := loadRoleEditor(a, r.PathValue("id"))
	if err != nil {
		failFragment(w, r, err)
		return
	}
	renderAll(w, r, components.AdminRoleContent(data))
}

func adminRoleSaveHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	id := r.PathValue("id")
	body := map[string]string{}
	if raw := r.Form.Get("name"); raw != r.Form.Get("loaded_name") {
		name := strings.TrimSpace(raw)
		if name == "" {
			failFragment(w, r, invalidInput("Enter a name"))
			return
		}
		body["name"] = name
	}
	if raw := r.Form.Get("description"); raw != r.Form.Get("loaded_description") {
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

// renderValueSaved answers a saved value with the granted list alone, since a value does not change which permissions can be granted.
func renderValueSaved(w http.ResponseWriter, r *http.Request, a adminAPI, drafts map[string]string) {
	data, err := loadRole(a, r.PathValue("id"))
	if err != nil {
		failFragment(w, r, afterWrite(err))
		return
	}
	data.Drafts = drafts
	data.FocusList = true
	renderAll(w, r, components.AdminRoleGranted(data), components.AdminStatus("admin-role-status", ""))
}

// renderGrantsChanged answers a grant or a removal with the granted list and the grant form, whose choices changed.
// The granted list keeps the drafts typed in it, and the grant form what was typed in it.
func renderGrantsChanged(w http.ResponseWriter, r *http.Request, a adminAPI, drafts map[string]string, grantPermission string, grantValue string, focusList bool) {
	data, err := loadRoleEditor(a, r.PathValue("id"))
	if err != nil {
		failFragment(w, r, afterWrite(err))
		return
	}
	data.Drafts = drafts
	data.GrantPermission = grantPermission
	data.GrantValue = grantValue
	data.FocusList = focusList || len(data.Available()) == 0
	renderAll(w, r, components.AdminRoleGranted(data), components.AdminRoleGrantForm(data, true), components.AdminStatus("admin-role-status", ""))
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
	if putGrant(w, r, a, r.Form.Get("grant_permission"), r.Form.Get("grant_value")) {
		renderGrantsChanged(w, r, a, roleDrafts(r.Form, ""), "", "", false)
	}
}

func adminRoleValueHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	permissionID := r.PathValue("permission")
	if putGrant(w, r, a, permissionID, r.Form.Get("value_"+permissionID)) {
		renderValueSaved(w, r, a, roleDrafts(r.Form, permissionID))
	}
}

func adminRoleRemoveHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	permissionID := r.PathValue("permission")
	path := "/roles/" + url.PathEscape(r.PathValue("id")) + "/permissions/" + url.PathEscape(permissionID)
	if _, err := adminSend[struct{}](a, http.MethodDelete, path, nil, "Failed to remove the permission"); err != nil {
		failFragment(w, r, err)
		return
	}
	renderGrantsChanged(w, r, a, roleDrafts(r.Form, permissionID), r.Form.Get("grant_permission"), r.Form.Get("grant_value"), true)
}

func adminRoleGrantValueHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	permission, err := findPermission(a, r.URL.Query().Get("grant_permission"))
	if err != nil {
		failFragment(w, r, err)
		return
	}
	renderAll(w, r, components.AdminGrantValue(permission, ""))
}

package main

import (
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
)

// maxGrantInt is the largest whole number the API accepts as a granted value, in either direction.
const maxGrantInt = 1 << 53

func roleFail(err error) *pageError {
	return pageFail(err).clearStatus(components.AdminRoleStatusID)
}

func adminRolesListHandler(w http.ResponseWriter, r *http.Request, a apiSession) error {
	roles, err := apiGet[[]components.Role](a, "/roles", loadRolesFailed)
	if err != nil {
		return err
	}
	renderAll(w, r, components.AdminRolesContent(components.AdminRolesData{Roles: roles}))
	return nil
}

func adminRoleCreateHandler(w http.ResponseWriter, r *http.Request, a apiSession) error {
	body := map[string]string{
		"name":        strings.TrimSpace(r.Form.Get("name")),
		"description": strings.TrimSpace(r.Form.Get("description")),
	}
	role, err := apiSend[components.Role](a, http.MethodPost, "/roles", body, "Failed to create the role")
	if err != nil {
		return pageFail(err).flagField(func(message string) templ.Component {
			return components.AdminRoleCreateName(r.Form.Get("name"), message, true)
		})
	}
	redirectHTMX(w, "/admin/roles/"+url.PathEscape(role.ID))
	return nil
}

func loadRole(a apiSession, id string) (components.AdminRoleData, error) {
	role, err := apiGet[components.Role](a, "/roles/"+url.PathEscape(id), "Failed to load the role")
	if err != nil {
		return components.AdminRoleData{}, err
	}
	return components.AdminRoleData{Role: role}, nil
}

func loadRoleEditor(a apiSession, id string) (components.AdminRoleData, error) {
	data, err := loadRole(a, id)
	if err != nil {
		return components.AdminRoleData{}, err
	}
	data.Catalogue, err = apiGet[[]components.Permission](a, "/permissions", loadPermissionsFailed)
	if err != nil {
		return components.AdminRoleData{}, err
	}
	return data, nil
}

func adminRoleEditorHandler(w http.ResponseWriter, r *http.Request, a apiSession) error {
	data, err := loadRoleEditor(a, r.PathValue("id"))
	if err != nil {
		return err
	}
	renderAll(w, r, components.AdminRoleContent(data))
	return nil
}

func adminRoleSaveHandler(w http.ResponseWriter, r *http.Request, a apiSession) error {
	id := r.PathValue("id")
	body := map[string]string{}
	if raw := r.Form.Get("name"); raw != r.Form.Get("loaded_name") {
		name := strings.TrimSpace(raw)
		if name == "" {
			return roleFail(invalidInput("Enter a name")).restore(components.AdminRoleName(raw, "Enter a name", true))
		}
		body["name"] = name
	}
	if raw := r.Form.Get("description"); raw != r.Form.Get("loaded_description") {
		body["description"] = strings.TrimSpace(raw)
	}
	if len(body) == 0 {
		nothingToSave(w, r, components.AdminRoleStatusID)
		return nil
	}
	role, err := apiSend[components.Role](a, http.MethodPatch, "/roles/"+url.PathEscape(id), body, "Failed to save the role")
	if err != nil {
		failure := roleFail(err)
		if body["name"] != "" {
			failure.flagField(func(message string) templ.Component {
				return components.AdminRoleName(r.Form.Get("name"), message, true)
			})
		}
		return failure
	}
	data := components.AdminRoleData{Role: role}
	renderAll(w, r,
		components.AdminRoleForm(data),
		components.AdminRoleHeader(data, true),
		components.AdminRoleDelete(data, true),
		components.StatusLine(components.AdminRoleStatusID, "Saved"),
	)
	return nil
}

func adminRoleDeleteHandler(w http.ResponseWriter, r *http.Request, a apiSession) error {
	if _, err := apiSend[struct{}](a, http.MethodDelete, "/roles/"+url.PathEscape(r.PathValue("id")), nil, "Failed to delete the role"); err != nil {
		return roleFail(err)
	}
	redirectHTMX(w, "/admin/roles")
	return nil
}

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

func findPermission(a apiSession, id string) (components.Permission, error) {
	catalogue, err := apiGet[[]components.Permission](a, "/permissions", loadPermissionsFailed)
	if err != nil {
		return components.Permission{}, err
	}
	for _, permission := range catalogue {
		if permission.ID == id {
			return permission, nil
		}
	}
	return components.Permission{}, &apiError{Status: http.StatusNotFound, Message: "Permission not found"}
}

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

func renderGrantsChanged(w http.ResponseWriter, r *http.Request, a apiSession, drafts map[string]string, grantPermission string, grantValue string, focusList bool, restore func() []templ.Component) error {
	data, err := loadRoleEditor(a, r.PathValue("id"))
	if err != nil {
		return roleFail(err).afterWrite().restoreLater(restore)
	}
	data.Drafts = drafts
	data.GrantPermission = grantPermission
	data.GrantValue = grantValue
	data.FocusList = focusList || len(data.Available()) == 0
	renderAll(w, r, components.AdminRoleGranted(data, false), components.AdminRoleGrantForm(data, true), components.StatusLine(components.AdminRoleStatusID, ""))
	return nil
}

func putGrant(r *http.Request, a apiSession, permissionID string, raw string) (components.Permission, error) {
	permission, err := findPermission(a, permissionID)
	if err != nil {
		return permission, roleFail(err)
	}
	body, err := grantBody(permission, raw)
	if err != nil {
		return permission, roleFail(err)
	}
	path := "/roles/" + url.PathEscape(r.PathValue("id")) + "/permissions/" + url.PathEscape(permissionID)
	if _, err := apiSend[struct{}](a, http.MethodPut, path, body, "Failed to grant the permission"); err != nil {
		return permission, roleFail(err)
	}
	return permission, nil
}

func adminRoleGrantHandler(w http.ResponseWriter, r *http.Request, a apiSession) error {
	permission, err := putGrant(r, a, r.Form.Get("grant_permission"), r.Form.Get("grant_value"))
	if err != nil {
		return err
	}
	drafts := roleDrafts(r.Form, "")
	return renderGrantsChanged(w, r, a, drafts, "", "", false, func() []templ.Component {
		return grantedWith(a, r, permission, drafts)
	})
}

// grantedWith draws the list and the form as they stand once permission is granted, for a grant whose reload failed.
func grantedWith(a apiSession, r *http.Request, granted components.Permission, drafts map[string]string) []templ.Component {
	catalogue, err := apiGet[[]components.Permission](a, "/permissions", loadPermissionsFailed)
	if err != nil {
		return []templ.Component{components.AdminRoleGrantUnavailable()}
	}
	data := components.AdminRoleData{Role: components.Role{ID: r.PathValue("id")}, Catalogue: catalogue, Drafts: drafts}
	for _, id := range append(slices.Clone(r.Form["granted"]), granted.ID) {
		for _, permission := range catalogue {
			if permission.ID == id {
				data.Role.Permissions = append(data.Role.Permissions, components.RolePermission{Permission: permission})
			}
		}
	}
	data.Drafts = maps.Clone(drafts)
	data.Drafts[granted.ID] = r.Form.Get("grant_value")
	return []templ.Component{components.AdminRoleGranted(data, true), components.AdminRoleGrantForm(data, true)}
}

func adminRoleValueHandler(w http.ResponseWriter, r *http.Request, a apiSession) error {
	permissionID := r.PathValue("permission")
	if _, err := putGrant(r, a, permissionID, r.Form.Get("value_"+permissionID)); err != nil {
		return err
	}
	data, err := loadRole(a, r.PathValue("id"))
	if err != nil {
		return roleFail(err).afterWrite()
	}
	data.Drafts = roleDrafts(r.Form, permissionID)
	data.FocusList = true
	renderAll(w, r, components.AdminRoleGranted(data, false), components.StatusLine(components.AdminRoleStatusID, ""))
	return nil
}

func adminRoleRemoveHandler(w http.ResponseWriter, r *http.Request, a apiSession) error {
	permissionID := r.PathValue("permission")
	path := "/roles/" + url.PathEscape(r.PathValue("id")) + "/permissions/" + url.PathEscape(permissionID)
	if _, err := apiSend[struct{}](a, http.MethodDelete, path, nil, "Failed to remove the permission"); err != nil {
		return roleFail(err)
	}
	return renderGrantsChanged(w, r, a, roleDrafts(r.Form, permissionID), r.Form.Get("grant_permission"), r.Form.Get("grant_value"), true, func() []templ.Component {
		return append(rowGone(components.AdminGrantRowPrefix, permissionID), grantFormWithout(a, r, permissionID)...)
	})
}

func grantFormWithout(a apiSession, r *http.Request, removed string) []templ.Component {
	catalogue, err := apiGet[[]components.Permission](a, "/permissions", loadPermissionsFailed)
	if err != nil {
		return []templ.Component{components.AdminRoleGrantUnavailable()}
	}
	role := components.Role{ID: r.PathValue("id")}
	for _, id := range r.Form["granted"] {
		if id != removed {
			role.Permissions = append(role.Permissions, components.RolePermission{Permission: components.Permission{ID: id}})
		}
	}
	return []templ.Component{components.AdminRoleGrantForm(components.AdminRoleData{
		Role:            role,
		Catalogue:       catalogue,
		GrantPermission: r.Form.Get("grant_permission"),
		GrantValue:      r.Form.Get("grant_value"),
	}, true)}
}

func adminRoleGrantValueHandler(w http.ResponseWriter, r *http.Request, a apiSession) error {
	permission, err := findPermission(a, r.URL.Query().Get("grant_permission"))
	if err != nil {
		return roleFail(err)
	}
	renderAll(w, r, components.AdminGrantValue(permission, ""))
	return nil
}

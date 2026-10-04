package main

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
)

func adminPermissionsListHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	permissions, err := adminGet[[]components.Permission](a, "/permissions", "Failed to load permissions")
	if err != nil {
		failFragment(w, r, err)
		return
	}
	renderAll(w, r, components.AdminPermissionsContent(components.AdminPermissionsData{Permissions: permissions}))
}

func loadPermissions(a adminAPI, focusList bool) (components.AdminPermissionsData, error) {
	permissions, err := adminGet[[]components.Permission](a, "/permissions", "Failed to load permissions")
	if err != nil {
		return components.AdminPermissionsData{}, err
	}
	return components.AdminPermissionsData{Permissions: permissions, FocusList: focusList}, nil
}

func adminPermissionCreateHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	body := map[string]string{
		"node":        strings.TrimSpace(r.Form.Get("node")),
		"description": strings.TrimSpace(r.Form.Get("description")),
	}
	valueType := r.Form.Get("value_type")
	if valueType != "" {
		body["value_type"] = valueType
	}
	if valueType == "int" {
		body["merge"] = r.Form.Get("merge")
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

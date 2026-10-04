package main

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
)

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

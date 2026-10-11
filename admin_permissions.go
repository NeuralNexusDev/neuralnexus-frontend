package main

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
)

func adminPermissionsListHandler(w http.ResponseWriter, r *http.Request, a apiSession) error {
	data, err := loadPermissions(a, false)
	if err != nil {
		return err
	}
	renderAll(w, r, components.AdminPermissionsContent(data))
	return nil
}

func loadPermissions(a apiSession, focusList bool) (components.AdminPermissionsData, error) {
	permissions, err := apiGet[[]components.Permission](a, "/permissions", loadPermissionsFailed)
	if err != nil {
		return components.AdminPermissionsData{}, err
	}
	return components.AdminPermissionsData{Permissions: permissions, FocusList: focusList}, nil
}

func adminPermissionCreateHandler(w http.ResponseWriter, r *http.Request, a apiSession) error {
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
	created, err := apiSend[components.Permission](a, http.MethodPost, "/permissions", body, "Failed to create the permission")
	if err != nil {
		return pageFail(err).flagField(func(message string) templ.Component {
			return components.AdminPermissionCreateNode(r.Form.Get("node"), message, true)
		})
	}
	data, err := loadPermissions(a, false)
	if err != nil {
		return pageFail(err).afterWrite().restore(components.AdminPermissionAdded(created), components.AdminPermissionForm(true))
	}
	renderAll(w, r, components.AdminPermissionList(data), components.AdminPermissionForm(true))
	return nil
}

func adminPermissionDeleteHandler(w http.ResponseWriter, r *http.Request, a apiSession) error {
	if _, err := apiSend[struct{}](a, http.MethodDelete, "/permissions/"+url.PathEscape(r.PathValue("id")), nil, "Failed to delete the permission"); err != nil {
		return err
	}
	data, err := loadPermissions(a, true)
	if err != nil {
		return pageFail(err).afterWrite().restore(rowGone(components.AdminPermissionRowPrefix, r.PathValue("id"))...)
	}
	renderAll(w, r, components.AdminPermissionList(data))
	return nil
}

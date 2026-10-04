package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
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

// adminAction serves a change made by htmx and parses its form for the handler.
// The HX-Request header is one a cross-site form cannot send.
func adminAction(handler func(http.ResponseWriter, *http.Request, adminAPI)) http.Handler {
	return adminRoute(func(w http.ResponseWriter, r *http.Request, a adminAPI) {
		if r.Header.Get("HX-Request") != "true" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		if err := r.ParseForm(); err != nil {
			failFragment(w, r, invalidInput("The form could not be read"))
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

// adminShell serves a page shell that loads its content. A shell holds no data, but it is not cached
// so a page left open after sign-out is not restored with what it loaded.
func adminShell(page templ.Component) http.Handler {
	return noStoreHandler(templ.Handler(page))
}

// adminShellFor serves the shell of a page for the record named by the ID in the path.
func adminShellFor(page func(id string) templ.Component) http.Handler {
	return noStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		templ.Handler(page(r.PathValue("id"))).ServeHTTP(w, r)
	}))
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
	segments := strings.Split(strings.TrimPrefix(path, "/admin/"), "/")
	if len(segments) < 2 {
		return ""
	}
	switch {
	case segments[0] == "users" && segments[1] != "list" && segments[1] != "rows":
		return "admin-user-status"
	case segments[0] == "roles" && segments[1] != "list":
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

// permissionLink answers a page's placeholder with the link when the account holds one of the permissions, and with nothing otherwise.
// The link is a convenience, so a failed lookup shows no link.
func permissionLink(link templ.Component, nodes ...string) func(http.ResponseWriter, *http.Request, adminAPI) {
	return func(w http.ResponseWriter, r *http.Request, a adminAPI) {
		permissions, err := adminGet[[]string](a, "/users/me/permissions", "Failed to load your permissions")
		if err != nil {
			return
		}
		for _, node := range nodes {
			if hasPermission(permissions, node) {
				templ.Handler(link).ServeHTTP(w, r)
				return
			}
		}
	}
}

func adminCardsHandler(w http.ResponseWriter, r *http.Request, a adminAPI) {
	permissions, err := adminGet[[]string](a, "/users/me/permissions", "Failed to load your permissions")
	if err != nil {
		failFragment(w, r, err)
		return
	}
	renderAll(w, r, components.AdminCards(components.AdminDashboardData{
		Users: hasPermission(permissions, "users.admin"),
		Roles: hasPermission(permissions, "roles.admin"),
	}))
}

// afterWrite words a failure to reload what was just changed so it is not mistaken for a failure to change it.
func afterWrite(err error) error {
	if err == nil || errors.Is(err, errUnauthorized) {
		return err
	}
	status, message := errorStatus(err)
	return &adminError{Status: status, Message: "The change was made, but the page could not be refreshed: " + message}
}

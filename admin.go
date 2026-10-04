package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
)

const (
	userStatusID = "admin-user-status"
	roleStatusID = "admin-role-status"
)

func noStoreHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", noStore)
		next.ServeHTTP(w, r)
	})
}

func adminRoute(handler func(http.ResponseWriter, *http.Request, adminAPI)) http.Handler {
	return noStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, adminAPI{r: r})
	}))
}

// adminAction requires the HX-Request header, which a cross-site form cannot send.
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

// adminShell is not cached, so a page left open after sign-out is not restored with what it loaded.
func adminShell(page templ.Component) http.Handler {
	return noStoreHandler(templ.Handler(page))
}

func adminShellFor(page func(id string) templ.Component) http.Handler {
	return noStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		templ.Handler(page(r.PathValue("id"))).ServeHTTP(w, r)
	}))
}

func failFragment(w http.ResponseWriter, r *http.Request, err error, restore ...templ.Component) {
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
	render(w, r, append([]templ.Component{components.AdminErrorText(message)}, restore...)...)
}

func failEditor(w http.ResponseWriter, r *http.Request, statusID string, err error, restore ...templ.Component) {
	failFragment(w, r, err, append([]templ.Component{components.AdminStatus(statusID, "")}, restore...)...)
}

func nothingToSave(w http.ResponseWriter, r *http.Request, statusID string) {
	w.Header().Set("HX-Reswap", "none")
	renderAll(w, r, components.AdminStatus(statusID, "Nothing to save"))
}

func render(w http.ResponseWriter, r *http.Request, parts ...templ.Component) {
	for _, part := range parts {
		if err := part.Render(r.Context(), w); err != nil {
			return
		}
	}
}

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

func permissionLink(link templ.Component, nodes ...string) func(http.ResponseWriter, *http.Request, adminAPI) {
	return func(w http.ResponseWriter, r *http.Request, a adminAPI) {
		permissions, err := adminGet[[]string](a, "/users/me/permissions", loadYourPermsFailed)
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
	permissions, err := adminGet[[]string](a, "/users/me/permissions", loadYourPermsFailed)
	if err != nil {
		failFragment(w, r, err)
		return
	}
	renderAll(w, r, components.AdminCards(components.AdminDashboardData{
		Users: hasPermission(permissions, "users.admin"),
		Roles: hasPermission(permissions, "roles.admin"),
	}))
}

func afterWrite(err error) error {
	if err == nil || errors.Is(err, errUnauthorized) {
		return err
	}
	status, message := errorStatus(err)
	return &adminError{Status: status, Message: "The change was made, but the page could not be refreshed: " + message}
}

func rowGone(prefix string, id string) []templ.Component {
	if id == "" || strings.Trim(id, "0123456789") != "" {
		return nil
	}
	return []templ.Component{components.AdminRowGone(prefix + id)}
}

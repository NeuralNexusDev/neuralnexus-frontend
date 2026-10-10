package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
)

func noStoreHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", noStore)
		next.ServeHTTP(w, r)
	})
}

var pageRequestTimeout = 30 * time.Second

type pageHandler interface {
	func(http.ResponseWriter, *http.Request, apiSession) | func(http.ResponseWriter, *http.Request, apiSession) error
}

func callHandler[H pageHandler](handler H, w http.ResponseWriter, r *http.Request, a apiSession) error {
	switch h := any(handler).(type) {
	case func(http.ResponseWriter, *http.Request, apiSession) error:
		return h(w, r, a)
	case func(http.ResponseWriter, *http.Request, apiSession):
		h(w, r, a)
	}
	return nil
}

func pageRoute[H pageHandler](handler H) http.Handler {
	return noStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), pageRequestTimeout)
		defer cancel()
		r = r.WithContext(ctx)
		if err := callHandler(handler, w, r, apiSession{r: r}); err != nil {
			writeError(w, r, err)
		}
	}))
}

func requireHTMXForWrites(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			mux.ServeHTTP(w, r)
			return
		}
		if _, pattern := mux.Handler(r); pattern == "" || pattern == "/" {
			mux.ServeHTTP(w, r)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" || site == "same-site" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		if r.Header.Get("HX-Request") != "true" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// pageAction requires the HX-Request header, which a cross-site form cannot send.
func pageAction[H pageHandler](handler H) http.Handler {
	return pageRoute(func(w http.ResponseWriter, r *http.Request, a apiSession) error {
		if r.Header.Get("HX-Request") != "true" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return nil
		}
		if err := r.ParseForm(); err != nil {
			return invalidInput("The form could not be read")
		}
		return callHandler(handler, w, r, a)
	})
}

// shell is not cached, so a page left open after sign-out is not restored with what it loaded.
func shell(page templ.Component) http.Handler {
	return noStoreHandler(templ.Handler(page))
}

func shellFor(page func(id string) templ.Component) http.Handler {
	return noStoreHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		templ.Handler(page(r.PathValue("id"))).ServeHTTP(w, r)
	}))
}

func failFragment(w http.ResponseWriter, r *http.Request, err error, restore ...templ.Component) {
	writeError(w, r, pageFail(err).restore(restore...))
}

func failEditor(w http.ResponseWriter, r *http.Request, statusID string, err error, restore ...templ.Component) {
	writeError(w, r, pageFail(err).clearStatus(statusID).restore(restore...))
}

func nothingToSave(w http.ResponseWriter, r *http.Request, statusID string) {
	w.Header().Set("HX-Reswap", "none")
	renderAll(w, r, components.StatusLine(statusID, "Nothing to save"))
}

func fieldRefusal(err error) string {
	var failure *apiError
	if errors.As(err, &failure) {
		switch failure.Status {
		case http.StatusBadRequest, http.StatusConflict, http.StatusUnprocessableEntity:
			return failure.Message
		}
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

func renderAll(w http.ResponseWriter, r *http.Request, parts ...templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, append(parts, components.ErrorClear())...)
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
	failure := pageFail(err)
	if failure.unauthorized() {
		return "", err
	}
	_, message := failure.statusMessage()
	return message, nil
}

func permissionLink(link templ.Component, nodes ...string) func(http.ResponseWriter, *http.Request, apiSession) {
	return func(w http.ResponseWriter, r *http.Request, a apiSession) {
		permissions, err := apiGet[[]string](a, "/users/me/permissions", loadYourPermsFailed)
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

func afterWrite(err error) error {
	if err == nil {
		return nil
	}
	return pageFail(err).afterWrite()
}

func rowGone(prefix string, id string) []templ.Component {
	if id == "" || strings.Trim(id, "0123456789") != "" {
		return nil
	}
	return []templ.Component{components.RowGone(prefix + id)}
}

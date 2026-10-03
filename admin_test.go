package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminPagesAreNotCached(t *testing.T) {
	router := NewWebServer("", false).Setup()
	cases := []struct{ path, marker string }{
		{"/admin", `id="admin-users-link"`},
		{"/admin/users", `id="admin-users-search"`},
		{"/admin/users/354102516314675901", `id="admin-user-form"`},
		{"/admin/roles", `id="admin-role-create-form"`},
		{"/admin/roles/354102516314675763", `id="admin-role-content"`},
		{"/admin/permissions", `id="admin-permission-create-form"`},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "no-store")
			}
			if !strings.Contains(rec.Body.String(), tc.marker) {
				t.Errorf("the page is missing %s", tc.marker)
			}
		})
	}
}

func TestOtherPagesKeepTheirCaching(t *testing.T) {
	router := NewWebServer("", false).Setup()
	for _, path := range []string{"/", "/account", "/projects"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rec.Header().Get("Cache-Control"); got != "" {
			t.Errorf("%s: Cache-Control = %q, want none", path, got)
		}
	}
}

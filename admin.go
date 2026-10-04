package main

import (
	"net/http"

	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
)

func adminCardsHandler(w http.ResponseWriter, r *http.Request, a apiSession) {
	permissions, err := apiGet[[]string](a, "/users/me/permissions", loadYourPermsFailed)
	if err != nil {
		failFragment(w, r, err)
		return
	}
	renderAll(w, r, components.AdminCards(components.AdminDashboardData{
		Users: hasPermission(permissions, "users.admin"),
		Roles: hasPermission(permissions, "roles.admin"),
	}))
}

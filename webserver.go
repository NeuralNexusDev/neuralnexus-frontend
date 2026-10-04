package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/a-h/templ"
	"github.com/p0t4t0sandwich/neuralnexus-frontend/components"
	mw "github.com/p0t4t0sandwich/neuralnexus-frontend/middleware"
)

// WebServer - The web server
type WebServer struct {
	Address  string
	UsingUDS bool
}

// NewWebServer - Create a new API server
func NewWebServer(address string, usingUDS bool) *WebServer {
	return &WebServer{
		Address:  address,
		UsingUDS: usingUDS,
	}
}

// Setup - Setup the web server
func (s *WebServer) Setup() http.Handler {
	router := http.NewServeMux()

	router.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))

	router.Handle("/", templ.Handler(components.HomePage()))
	router.Handle("/login", templ.Handler(components.LoginPage()))
	router.Handle("/register", templ.Handler(components.RegisterPage()))
	router.Handle("/account", shell(components.AccountPage()))
	router.Handle("GET /account/content", pageRoute(accountContentHandler))
	router.Handle("POST /account/settings", pageAction(accountSettingsHandler))
	router.Handle("POST /account/links/{platform}", pageAction(accountLinkHandler))
	router.Handle("DELETE /account/links/{platform}", pageAction(accountUnlinkHandler))
	router.Handle("GET /admin", shell(components.AdminDashboardPage()))
	router.Handle("GET /admin/cards", pageRoute(adminCardsHandler))
	router.Handle("GET /admin/users", shell(components.AdminUsersPage()))
	router.Handle("GET /admin/users/list", pageRoute(adminUsersListHandler))
	router.Handle("GET /admin/users/rows", pageRoute(adminUserRowsHandler))
	router.Handle("GET /admin/users/{id}", shellFor(components.AdminUserPage))
	router.Handle("GET /admin/users/{id}/editor", pageRoute(adminUserEditorHandler))
	router.Handle("POST /admin/users/{id}", pageAction(adminUserSaveHandler))
	router.Handle("GET /admin/roles", shell(components.AdminRolesPage()))
	router.Handle("GET /admin/roles/list", pageRoute(adminRolesListHandler))
	router.Handle("POST /admin/roles", pageAction(adminRoleCreateHandler))
	router.Handle("GET /admin/roles/{id}", shellFor(components.AdminRolePage))
	router.Handle("GET /admin/roles/{id}/editor", pageRoute(adminRoleEditorHandler))
	router.Handle("POST /admin/roles/{id}", pageAction(adminRoleSaveHandler))
	router.Handle("DELETE /admin/roles/{id}", pageAction(adminRoleDeleteHandler))
	router.Handle("GET /admin/roles/{id}/grant-value", pageAction(adminRoleGrantValueHandler))
	router.Handle("POST /admin/roles/{id}/permissions", pageAction(adminRoleGrantHandler))
	router.Handle("POST /admin/roles/{id}/permissions/{permission}", pageAction(adminRoleValueHandler))
	router.Handle("DELETE /admin/roles/{id}/permissions/{permission}", pageAction(adminRoleRemoveHandler))
	router.Handle("GET /admin/permissions", shell(components.AdminPermissionsPage()))
	router.Handle("GET /admin/permissions/list", pageRoute(adminPermissionsListHandler))
	router.Handle("POST /admin/permissions", pageAction(adminPermissionCreateHandler))
	router.Handle("DELETE /admin/permissions/{id}", pageAction(adminPermissionDeleteHandler))
	router.Handle("/projects", templ.Handler(components.ProjectsPage()))
	router.Handle("/project/bee-name-generator", templ.Handler(components.BeeNameGeneratorPage()))
	router.Handle("GET /project/bee-name-generator/admin-link", pageRoute(permissionLink(components.BeeNameGeneratorAdminLink(), "beenamegenerator.admin")))
	router.Handle("GET /project/bee-name-generator/admin", shell(components.BeeNameGeneratorAdminPage()))
	router.Handle("GET /project/bee-name-generator/admin/suggestions", pageRoute(beeSuggestionsHandler))
	router.Handle("POST /project/bee-name-generator/admin/suggestions", pageAction(beeReviewHandler))
	router.HandleFunc("GET /project/mc-status", McStatusPageHandler)
	router.HandleFunc("GET /project/mc-status/{host}", McStatusPageHandler)
	router.Handle("/teapot", templ.Handler(components.TeapotPage()))

	middlewareStack := mw.CreateStack(
		mw.RecoveryMiddleware,
		mw.SecurityHeadersMiddleware,
		mw.SessionMiddleware,
		mw.RequestIDMiddleware,
		mw.IPMiddleware,
		mw.RequestLoggerMiddleware,
	)

	return middlewareStack(requireHTMXForWrites(router))
}

// Run - Start the web server
func (s *WebServer) Run() error {
	server := http.Server{
		Addr:              s.Address,
		Handler:           s.Setup(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	if s.UsingUDS {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			os.Remove(s.Address)
			os.Exit(1)
		}()

		if _, err := os.Stat(s.Address); err == nil {
			log.Printf("Removing existing socket file %s", s.Address)
			if err := os.Remove(s.Address); err != nil {
				return err
			}
		}

		socket, err := net.Listen("unix", s.Address)
		if err != nil {
			return err
		}

		log.Printf("WebServer listening on %s", s.Address)
		return server.Serve(socket)
	} else {
		log.Printf("WebServer listening on %s", s.Address)
		return server.ListenAndServe()
	}
}

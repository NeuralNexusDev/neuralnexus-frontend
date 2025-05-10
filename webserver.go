package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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
	router.Handle("/projects", templ.Handler(components.ProjectsPage()))
	router.Handle("/project/bee-name-generator", templ.Handler(components.BeeNameGeneratorPage()))
	router.Handle("/teapot", templ.Handler(components.TeapotPage()))

	middlewareStack := mw.CreateStack(
		mw.SessionMiddleware,
		mw.RequestIDMiddleware,
		mw.IPMiddleware,
		mw.RequestLoggerMiddleware,
	)

	return middlewareStack(router)
}

// Run - Start the web server
func (s *WebServer) Run() error {
	server := http.Server{
		Addr:    s.Address,
		Handler: s.Setup(),
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

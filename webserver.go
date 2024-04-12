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
	"github.com/p0t4t0sandwich/neuralnexus-frontend/middleware"
)

// -------------- Structs --------------
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

// Run - Start the web server
func (s *WebServer) Run() error {
	router := http.NewServeMux()

	// Static public files
	router.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))

	router.Handle("/", templ.Handler(components.HomePage()))
	router.Handle("/login", templ.Handler(components.LoginPage()))
	router.Handle("/projects", templ.Handler(components.ProjectsPage()))
	router.Handle("/project/bee-name-generator", templ.Handler(components.BeeNameGeneratorPage()))

	server := http.Server{
		Addr:    s.Address,
		Handler: middleware.RequestLoggerMiddleware(router),
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

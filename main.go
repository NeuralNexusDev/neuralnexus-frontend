package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// -------------- Middleware --------------

// WrappedWriter - Wrapper for http.ResponseWriter
type WrappedWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader - Write the header
func (w *WrappedWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.statusCode = statusCode
}

// Middleware - Middleware type
type Middleware func(http.Handler) http.Handler

// CreateStack - Create a stack of middlewares
func CreateStack(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// RequestLoggerMiddleware - Log all requests
func RequestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &WrappedWriter{w, http.StatusOK}
		next.ServeHTTP(wrapped, r)

		cfConnectingIP := r.Header.Get("CF-Connecting-IP")
		forwardedFor := r.Header.Get("X-Forwarded-For")
		if cfConnectingIP != "" {
			r.RemoteAddr = cfConnectingIP
		} else if forwardedFor != "" {
			r.RemoteAddr = forwardedFor
		}

		log.Printf("%s %d %s %s %s", r.RemoteAddr, wrapped.statusCode, r.Method, r.URL.Path, time.Since(start))
	})
}

// -------------- Main --------------
func main() {
	ip := os.Getenv("IP_ADDRESS")
	if ip == "" {
		ip = "0.0.0.0"
	}
	port := os.Getenv("REST_PORT")
	if port == "" {
		port = "3022"
	}

	router := http.NewServeMux()
	router.Handle("/", http.FileServer(http.Dir("/static")))
	router.HandleFunc("/mcstatus/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "/static/mcstatus.html")
	})
	router.HandleFunc("/{path}", func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("path")
		var servePath string = "/static/" + path
		if !strings.Contains(path, ".") {
			servePath += ".html"
		}
		http.ServeFile(w, r, servePath)
	})

	middlewareStack := CreateStack(RequestLoggerMiddleware)

	server := http.Server{
		Addr:    ip + ":" + port,
		Handler: middlewareStack(router),
	}
	log.Fatal(server.ListenAndServe())
}

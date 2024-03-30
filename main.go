package main

import (
	"log"
	"net/http"
	"os"
)

// -------------- Globals --------------

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
	router.HandleFunc("/mcstatus/", mcstatusHandler)

	server := http.Server{
		Addr:    ip + ":" + port,
		Handler: router,
	}
	log.Fatal(server.ListenAndServe())
}

// -------------- Handlers --------------
func mcstatusHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "/static/mcstatus.html")
}

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
	router.HandleFunc("/mcstatus/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "/static/mcstatus.html")
	})
	router.HandleFunc("/bee-name-generator", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "/static/bee-name-generator.html")
	})

	server := http.Server{
		Addr:    ip + ":" + port,
		Handler: router,
	}
	log.Fatal(server.ListenAndServe())
}

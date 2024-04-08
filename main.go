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
	// router.HandleFunc("/bee-name-generator", func(w http.ResponseWriter, r *http.Request) {
	// 	http.ServeFile(w, r, "/static/bee-name-generator.html")
	// })
	// Handle the rest
	router.HandleFunc("/{rest:.*}", func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("rest")
		// if strings.Contains(path, ".") || strings.Contains(path, "/") {
		// 	http.Error(w, "404 page not found", http.StatusNotFound)
		// 	return
		// }
		http.ServeFile(w, r, "/static/"+path+".html")
	})

	server := http.Server{
		Addr:    ip + ":" + port,
		Handler: router,
	}
	log.Fatal(server.ListenAndServe())
}

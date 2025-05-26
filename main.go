package main

import (
	"log"
	"os"
)

func main() {
	address := os.Getenv("ADDRESS")
	useUDS := os.Getenv("USE_UDS") == "true"
	if address == "" && useUDS {
		address = "/tmp/go.socket"
	} else if address == "" {
		address = "0.0.0.0:8090"
	}

	server := NewWebServer(address, useUDS)
	log.Fatal(server.Run())
}

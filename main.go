package main

import (
	"log"
	"os"
)

func listenAddress(address string, useUDS bool) string {
	if address != "" {
		return address
	}
	if useUDS {
		return "/tmp/go.socket"
	}
	return "0.0.0.0:8090"
}

func main() {
	useUDS := os.Getenv("USE_UDS") == "true"
	server := NewWebServer(listenAddress(os.Getenv("ADDRESS"), useUDS), useUDS)
	log.Fatal(server.Run())
}

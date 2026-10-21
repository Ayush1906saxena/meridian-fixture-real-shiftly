package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	addr := ":" + envOr("PORT", "8080")
	log.Printf("Shiftly on %s", addr)
	log.Fatal(http.ListenAndServe(addr, NewApp(NewStore()).Handler()))
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

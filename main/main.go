package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/handlers"
	"github.com/hiconvo/api/utils/secrets"
)

func main() {
	cp := db.Client
	secrets.Init(cp)
	http.Handle("/", handlers.CreateRouter())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

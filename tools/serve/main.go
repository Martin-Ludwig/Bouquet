package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Println("Serving ./web on http://localhost:" + port)
	log.Fatal(http.ListenAndServe(":"+port, http.FileServer(http.Dir("web"))))
}

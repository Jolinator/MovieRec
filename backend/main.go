package main

import (
	"log"
	"net/http"
)

func main() {

	mux := http.NewServeMux()
	RegisterEndpoints(mux)

	log.Println("API running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

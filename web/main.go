package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
)

func main() {
	router := mux.NewRouter()

	router.PathPrefix("/").HandlerFunc(maybeRedirect)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	http.Handle("/", router)

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func maybeRedirect(w http.ResponseWriter, r *http.Request) {
	ext := filepath.Ext(r.URL.Path)

	// Send them to the 404 page if it looks like a page was being requested.
	// Otherwise, 404.
	if ext == "html" || ext == "htm" || ext == "" {
		http.Redirect(w, r, "/404.html", http.StatusFound)
	} else {
		http.NotFound(w, r)
	}
}

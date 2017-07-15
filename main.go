package main

import (
	"net/http"
	"path/filepath"
)

func init() {
	http.HandleFunc("/", redirect)
}

func redirect(w http.ResponseWriter, r *http.Request) {
	ext := filepath.Ext(r.URL.Path)

	// Send them to the 404 page if it looks like a page was being requested.
	// Otherwise, 404.
	if ext == "html" || ext == "htm" || ext == "" {
		http.Redirect(w, r, "/404.html", http.StatusFound)
	} else {
		http.NotFound(w, r)
	}
}

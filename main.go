package main

import (
	"fmt"
	"net/http"
)

func main() {
	fmt.Println("hello world")
	http.Handle("/static/", http.FileServer(http.Dir(".")))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/static/", http.StatusMovedPermanently)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Listen failed:", err)
	}
}

package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.Handle("/static/", http.FileServer(http.Dir(".")))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/static/", http.StatusMovedPermanently)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	http.HandleFunc("/rest/add_wrong_word", handleAddWrongWord)
	http.HandleFunc("/rest/get_next_question", getNextQuestion)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Listen failed:", err)
	}
}

func getNextQuestion(w http.ResponseWriter, r *http.Request) {
	if getNextChoiceQuestion(w, r) {
		return
	}
	w.Write([]byte(`{"question": "已完成所有题目"}`))
}

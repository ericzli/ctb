package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

var g_Conf struct {
	ListenAddr string `json:"listen_addr"`
	DbUri      string `json:"mysql_database"`
}

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

	g_Conf.ListenAddr = ":8080"
	g_Conf.DbUri = "ctb:pass@tcp(localhost:8081)/ctb"
	b, err := ioutil.ReadFile("conf.json")
	if err == nil {
		err = json.Unmarshal(b, &g_Conf)
		if err != nil {
			fmt.Println("Unmarshal conf failed:", err)
		}
	}

	err = http.ListenAndServe(g_Conf.ListenAddr, nil)
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

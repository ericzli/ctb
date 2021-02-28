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
	http.HandleFunc("/rest/register_or_login", handleRegisterOrLogin)
	http.HandleFunc("/rest/add_choice_question", handleAddChoiceQuestion)
	http.HandleFunc("/rest/get_next_questions", getNextQuestions)
	http.HandleFunc("/rest/list_learning", listLearningQuestion)
	http.HandleFunc("/rest/update_learn_status", updateLearnStatus)

	g_Conf.ListenAddr = ":8080"
	g_Conf.DbUri = "ctb:pass@tcp(localhost:8081)/ctb"
	b, err := ioutil.ReadFile("conf.json")
	if err == nil {
		err = json.Unmarshal(b, &g_Conf)
		if err != nil {
			fmt.Println("Unmarshal conf failed:", err)
		}
	}
	initDb(g_Conf.DbUri)

	err = http.ListenAndServe(g_Conf.ListenAddr, nil)
	if err != nil {
		fmt.Println("Listen failed:", err)
	}
}

func listLearningQuestion(w http.ResponseWriter, r *http.Request) {
	defer errRecover4Rest(w)

	var rsp struct {
		Result    string        `json:"result"`
		Questions []interface{} `json:"questions"`
	}
	rsp.Questions = append(rsp.Questions, getLearningChoiceQuestions(r)...)
	rsp.Result = "ok"
	b, err := json.Marshal(&rsp)
	if err != nil {
		panic(fmt.Sprintf("marshal json failed: %v", err))
	}
	w.Write(b)
}

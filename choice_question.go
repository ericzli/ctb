package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

var s_DB *sql.DB

func checkDb() {
	if s_DB != nil {
		return
	}
	DB, err := sql.Open("mysql", g_Conf.DbUri)
	if err != nil || DB == nil {
		panic(fmt.Sprintf("Connect mysql failed, %v", err))
	}
	s_DB = DB
}

/*
CREATE TABLE `ctb_choice_question` (
	`id` int(11) NOT NULL AUTO_INCREMENT,
	`type` bigint(20) NOT NULL,
	`question` varchar(256) NOT NULL,
	`right_answer` varchar(256) NOT NULL,
	`wrong_answer` varchar(256) NOT NULL,
	PRIMARY KEY (`id`)
) ENGINE=MyISAM AUTO_INCREMENT=8 DEFAULT CHARSET=utf8;

CREATE TABLE `ctb_answer_record` (
	`question_id` int(11) NOT NULL,
	`user` char(16) NOT NULL,
	`rest_cnt` int(11) NOT NULL,
	`next_time` datetime NOT NULL,
	PRIMARY KEY (`question_id`,`user`)
) ENGINE=MyISAM DEFAULT CHARSET=utf8;
*/

func handleAddWrongWord(w http.ResponseWriter, r *http.Request) {
	checkDb()
	defer func() {
		err := recover()
		if err != nil {
			w.Write([]byte(fmt.Sprintf("添加失败：%v", err)))
		}
	}()

	user := r.PostFormValue("user")
	question := r.PostFormValue("question")
	rightAnswer := r.PostFormValue("right_answer")
	wrongAnswer := r.PostFormValue("wrong_answer")

	if !strings.Contains(question, "??") {
		panic("Question should contains '??'")
	}
	if rightAnswer == "" || wrongAnswer == "" {
		panic("Right or wrong answer should not be empty")
	}

	result, err := s_DB.Exec("insert INTO ctb_choice_question(type,question,right_answer,wrong_answer) values(1,?,?,?)", question, rightAnswer, wrongAnswer)
	if err != nil {
		panic(fmt.Sprintf("insert question failed, %v", err))
	}
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		panic(fmt.Sprintf("get lastInsertID failed, %v", err))
	}
	fmt.Println("Insert choice question success, id =", lastInsertID)
	_, err = s_DB.Exec("insert INTO ctb_answer_record(question_id,user,rest_cnt,next_time) values(?,?,5,now())", lastInsertID, user)
	if err != nil {
		panic(fmt.Sprintf("insert record failed, %v", err))
	}

	http.Redirect(w, r, "/static/add_wrong_word.html", http.StatusFound)
}

func getNextChoiceQuestion(w http.ResponseWriter, r *http.Request) bool {
	checkDb()
	user := r.FormValue("user")
	row := s_DB.QueryRow("select question_id from ctb_answer_record where user = ? and now() > next_time order by next_time limit 1", user)
	questionId := -1
	if err := row.Scan(&questionId); err != nil || questionId < 0 {
		return false
	}
	row = s_DB.QueryRow("select question, right_answer, wrong_answer from ctb_choice_question where id = ?", questionId)
	var rsp struct {
		Id          int      `json:"id"`
		Question    string   `json:"question"`
		Choices     []string `json:"choices"`
		RightAnswer string
		WrongAnswer string
	}
	var rightAnswer, wrongAnswer string
	if err := row.Scan(&rsp.Question, &rightAnswer, &wrongAnswer); err != nil {
		panic(fmt.Sprintf("question %d not exists", questionId))
	}
	rsp.Id = questionId
	rsp.Choices = append(strings.Split(rightAnswer, ","), strings.Split(wrongAnswer, ",")...)
	shuffleStrings(rsp.Choices)
	b, err := json.Marshal(&rsp)
	if err != nil {
		panic(fmt.Sprintf("marshal json failed: %v", err))
	}
	w.Write(b)
	return true
}

func shuffleStrings(cards []string) {
	var size int = len(cards)
	var j int = 0

	for i, _ := range cards {
		j = rand.Intn(size-i) + i
		cards[i], cards[j] = cards[j], cards[i]
	}
}

func submitAnswer(w http.ResponseWriter, r *http.Request) {
	checkDb()
	id := r.FormValue("id")
	user := r.FormValue("user")
	chosen := strings.Split(r.FormValue("chosen"), ",")
	sort.Strings(chosen)
	row := s_DB.QueryRow("select right_answer from ctb_choice_question where id = ?", id)
	var correctAnswer string
	if err := row.Scan(&correctAnswer); err != nil {
		panic("question not exist")
	}
	correct := strings.Split(correctAnswer, ",")
	sort.Strings(correct)
	var rsp struct {
		Correct bool   `json:"correct"`
		Detail  string `json:"detail"`
	}
	if len(chosen) == len(correct) {
		rsp.Correct = true
		for i := range chosen {
			if chosen[i] != correct[i] {
				rsp.Correct = false
			}
		}
	}
	rsp.Detail = fmt.Sprintf("答案：%s", correctAnswer)
	b, err := json.Marshal(&rsp)
	if err != nil {
		panic(fmt.Sprintf("marshal json failed: %v", err))
	}
	w.Write(b)

	// 另起协程让页面响应更快
	go func() {
		if rsp.Correct {
			_, err := s_DB.Exec("update ctb_answer_record set rest_cnt = rest_cnt - 1, next_time = date_add(now(), interval 12 hour) where question_id = ? and user = ?",
				id, user)
			if err != nil {
				fmt.Printf("update answer record failed, %v\n", err)
			}
			_, err = s_DB.Exec("delete from ctb_answer_record where question_id = ? and rest_cnt <= 0", id)
			if err != nil {
				fmt.Printf("clear answer record failed, %v\n", err)
			}
		} else {
			_, err := s_DB.Exec("update ctb_answer_record set rest_cnt = rest_cnt + 5, next_time = date_add(now(), interval 1 minute) where question_id = ? and user = ?",
				id, user)
			if err != nil {
				fmt.Printf("update answer record failed, %v\n", err)
			}
		}
	}()
}

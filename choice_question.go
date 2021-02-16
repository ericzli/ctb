package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"runtime/debug"
	"sort"
	"strconv"
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
			debug.PrintStack()
			w.Write([]byte(fmt.Sprintf("添加失败：%v", err)))
		}
	}()

	user := r.PostFormValue("user")
	question := ReplaceAllPinyin(r.PostFormValue("question"), "??(", ")")
	rightAnswer := strings.Replace(strings.Replace(r.PostFormValue("right_answer"), "，", ",", -1), " ", ",", -1)
	wrongAnswer := strings.Replace(strings.Replace(r.PostFormValue("wrong_answer"), "，", ",", -1), " ", ",", -1)
	restCount, err := strconv.Atoi(r.PostFormValue("rest_count"))
	if err != nil || restCount < 0 {
		panic("invalid rest_count")
	}

	if question == "" {
		panic("question should not be empty")
	}
	if rightAnswer == "" || wrongAnswer == "" {
		panic("iight or wrong answer should not be empty")
	}
	row := s_DB.QueryRow("select id, wrong_answer from ctb_choice_question where question = ? limit 1", question)
	questionId := -1
	var oldRightChoice string
	row.Scan(&questionId, &oldRightChoice)
	if questionId < 0 {
		result, err := s_DB.Exec("insert INTO ctb_choice_question(type,question,right_answer,wrong_answer) values(1,?,?,?)", question, rightAnswer, wrongAnswer)
		if err != nil {
			panic(fmt.Sprintf("insert question failed, %v", err))
		}
		lastInsertID, err := result.LastInsertId()
		if err != nil {
			panic(fmt.Sprintf("get lastInsertID failed, %v", err))
		}
		fmt.Printf("Insert choice question success, id = %d, user = %s\n", lastInsertID, user)
		_, err = s_DB.Exec("insert INTO ctb_answer_record(question_id,user,rest_cnt,next_time) values(?,?,?,now())", lastInsertID, user, restCount)
		if err != nil {
			panic(fmt.Sprintf("insert record failed, %v", err))
		}
	} else {
		// 合并干扰项
		wrongAnswer = strings.Join(removeRepeatedElement(append(strings.Split(wrongAnswer, ","), strings.Split(oldRightChoice, ",")...)), ",")
		_, err := s_DB.Exec("update ctb_choice_question set right_answer = ?, wrong_answer = ? where id = ?", rightAnswer, wrongAnswer, questionId)
		if err != nil {
			panic(fmt.Sprintf("update question failed, %v", err))
		}
		// 如果已存在，就增加额外次数
		row = s_DB.QueryRow("select count(0) from ctb_answer_record where question_id = ? and user = ?", questionId, user)
		var count int
		row.Scan(&count)
		if count > 0 {
			_, err = s_DB.Exec("update ctb_answer_record set rest_cnt = rest_cnt + ? where question_id = ? and user = ?", restCount, questionId, user)
		} else {
			_, err = s_DB.Exec("insert INTO ctb_answer_record(question_id,user,rest_cnt,next_time) values(?,?,?,now())", questionId, user, restCount)
		}
		if err != nil {
			panic(err)
		}
		fmt.Printf("Add to question exists, id = %d, user = %s\n", questionId, user)
	}

	http.Redirect(w, r, "/static/add_wrong_word.html", http.StatusFound)
}

func removeRepeatedElement(arr []string) (newArr []string) {
	newArr = make([]string, 0)
	for i := 0; i < len(arr); i++ {
		repeat := false
		for j := i + 1; j < len(arr); j++ {
			if arr[i] == arr[j] {
				repeat = true
				break
			}
		}
		if !repeat {
			newArr = append(newArr, arr[i])
		}
	}
	return
}

func getNextChoiceQuestion(r *http.Request) (int, interface{}) {
	checkDb()
	user := r.FormValue("user")
	var rsp struct {
		Id          int      `json:"id"`
		Question    string   `json:"question"`
		Choices     []string `json:"choices"`
		RightAnswer string
		WrongAnswer string
	}

	row := s_DB.QueryRow("select question_id from ctb_answer_record where user = ? and now() > next_time order by next_time limit 1", user)
	if err := row.Scan(&rsp.Id); err != nil {
		return 0, nil
	}
	row = s_DB.QueryRow("select count(0) from ctb_answer_record where user = ? and now() > next_time", user)
	var restCount int
	if err := row.Scan(&restCount); err != nil {
		panic("get rest count failed")
	}
	row = s_DB.QueryRow("select question, right_answer, wrong_answer from ctb_choice_question where id = ?", rsp.Id)
	if err := row.Scan(&rsp.Question, &rsp.RightAnswer, &rsp.WrongAnswer); err != nil {
		panic(fmt.Sprintf("question %d not exists", rsp.Id))
	}
	rsp.Choices = append(strings.Split(rsp.RightAnswer, ","), strings.Split(rsp.WrongAnswer, ",")...)
	shuffleStrings(rsp.Choices)
	return restCount, rsp
}

func shuffleStrings(cards []string) {
	var size int = len(cards)
	var j int = 0

	for i, _ := range cards {
		j = rand.Intn(size-i) + i
		cards[i], cards[j] = cards[j], cards[i]
	}
}

func getLearningChoiceQuestions(r *http.Request) []interface{} {
	checkDb()
	user := r.FormValue("user")
	result := []interface{}{}

	rows, err := s_DB.Query("select question_id, question, rest_cnt from ctb_answer_record, ctb_choice_question where id = question_id and user = ? limit 1000", user)
	if rows != nil {
		defer rows.Close()
	}
	if err != nil {
		panic("query learning question list from db failed")
	}
	for rows.Next() {
		var id, cnt int
		var question string
		err = rows.Scan(&id, &question, &cnt)
		if err != nil {
			panic(fmt.Sprintf("scan failed: %v", err))
		}
		result = append(result, &struct {
			Id        int    `json:"id"`
			Question  string `json:"question"`
			RestCount int    `json:"rest_count"`
		}{id, question, cnt})
	}
	return result
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
			_, err := s_DB.Exec("update ctb_answer_record set rest_cnt = rest_cnt + 5, next_time = now() where question_id = ? and user = ?",
				id, user)
			if err != nil {
				fmt.Printf("update answer record failed, %v\n", err)
			}
		}
	}()
}

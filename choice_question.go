package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"runtime/debug"
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

func handleAddChoiceQuestion(w http.ResponseWriter, r *http.Request) {
	checkDb()
	defer func() {
		err := recover()
		var rsp struct {
			Result string `json:"result"`
		}
		if err != nil {
			debug.PrintStack()
			rsp.Result = fmt.Sprintf("add failed: %v", err)
		} else {
			rsp.Result = "ok"
		}
		encoder := json.NewEncoder(w)
		encoder.Encode(&rsp)
	}()

	user := r.FormValue("user")

	body, err := ioutil.ReadAll(r.Body)
	check(err)

	var req struct {
		Type        int      `json:"type"`
		Question    string   `json:"question"`
		RightAnswer []string `json:"right_answer"`
		WrongAnswer []string `json:"wrong_answer"`
		RestCount   int      `json:"rest_count"`
	}
	check(json.Unmarshal(body, &req))

	if req.Type == 1 { // 别字题需要将拼音换成田字格
		req.Question = ReplaceAllPinyin(req.Question, "??(", ")")
	}
	if req.RestCount < 0 || req.RestCount > 1000 {
		panic("invalid rest_count")
	}

	if req.Question == "" || len(req.RightAnswer) == 0 {
		panic("question or right answer should not be empty")
	}

	for i, v := range req.RightAnswer {
		req.RightAnswer[i] = strings.Replace(v, ",", "，", -1) // 半角逗号用于分隔符，所以原本的半角逗号都替换成全角的
	}
	for i, v := range req.WrongAnswer {
		req.WrongAnswer[i] = strings.Replace(v, ",", "，", -1)
	}
	rightAnswer := strings.Join(req.RightAnswer, ",")
	wrongAnswer := strings.Join(req.WrongAnswer, ",")

	// 检查问题是否已存在
	questionId := -1
	var oldRightChoice string
	if req.Type == 1 {
		row := s_DB.QueryRow("select id, wrong_answer from ctb_choice_question where question = ? limit 1", req.Question)
		row.Scan(&questionId, &oldRightChoice)
	}
	if questionId < 0 {
		// 如果不存在则添加新题
		result, err := s_DB.Exec("insert INTO ctb_choice_question(type,question,right_answer,wrong_answer) values(?,?,?,?)",
			req.Type, req.Question, rightAnswer, wrongAnswer)
		checkf(err, "insert question failed")
		lastInsertID, err := result.LastInsertId()
		checkf(err, "get lastInsertID failed")
		fmt.Printf("Insert choice question success, id = %d, user = %s\n", lastInsertID, user)
		_, err = s_DB.Exec("insert INTO ctb_answer_record(question_id,user,rest_cnt,next_time) values(?,?,?,now())", lastInsertID, user, req.RestCount)
		checkf(err, "insert record failed")
	} else {
		// 如果已存在则合并干扰项
		wrongAnswer = strings.Join(removeRepeatedElement(append(strings.Split(wrongAnswer, ","), strings.Split(oldRightChoice, ",")...)), ",")
		_, err := s_DB.Exec("update ctb_choice_question set right_answer = ?, wrong_answer = ? where id = ?", rightAnswer, wrongAnswer, questionId)
		checkf(err, "update question failed")
		// 如果当前正在学习此题，就增加额外次数
		row := s_DB.QueryRow("select count(0) from ctb_answer_record where question_id = ? and user = ?", questionId, user)
		var count int
		row.Scan(&count)
		if count > 0 {
			_, err = s_DB.Exec("update ctb_answer_record set rest_cnt = rest_cnt + ? where question_id = ? and user = ?", req.RestCount, questionId, user)
		} else {
			_, err = s_DB.Exec("insert INTO ctb_answer_record(question_id,user,rest_cnt,next_time) values(?,?,?,now())", questionId, user, req.RestCount)
		}
		check(err)
		fmt.Printf("Add to question exists, id = %d, user = %s\n", questionId, user)
	}
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
		Type        int      `json:"type"`
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
	row = s_DB.QueryRow("select type, question, right_answer, wrong_answer from ctb_choice_question where id = ?", rsp.Id)
	if err := row.Scan(&rsp.Type, &rsp.Question, &rsp.RightAnswer, &rsp.WrongAnswer); err != nil {
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
	checkf(row.Scan(&correctAnswer), "question not exist")
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
	checkf(err, "marshal json failed")
	w.Write(b)

	// 另起协程让页面响应更快
	go func() {
		if rsp.Correct {
			// 按记忆曲线更新下次学习的时间
			_, err := s_DB.Exec(`update ctb_answer_record set rest_cnt = rest_cnt - 1, next_time = date_add(now(), interval (case rest_cnt
					when 1 then 90*24*60
					when 2 then 45*24*60
					when 3 then 20*24*60
					when 4 then 10*24*60
					when 5 then 5*24*60
					when 6 then 2*24*60
					when 7 then 24*60
					when 8 then 10*60
					when 9 then 5*60
					when 10 then 2*60
					when 11 then 60
					when 12 then 30
					when 13 then 12
					else 5 end
				) minute) where question_id = ? and user = ?`, id, user)
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

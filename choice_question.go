package main

import (
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

func handleAddChoiceQuestion(w http.ResponseWriter, r *http.Request) {
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

	user_id := getUserId(r)

	body, err := ioutil.ReadAll(r.Body)
	check(err)

	var req struct {
		AddType     string   `json:"add_type"`
		Question    string   `json:"question"`
		RightAnswer []string `json:"right_answer"`
		WrongAnswer []string `json:"wrong_answer"`
		RestCount   int      `json:"rest_count"`
	}
	check(json.Unmarshal(body, &req))
	if req.RestCount < 0 || req.RestCount > 1000 {
		panic("invalid rest_count")
	}
	if req.Question == "" || len(req.RightAnswer) == 0 {
		panic("question or right answer should not be empty")
	}

	// 半角逗号用于分隔符，所以原本的半角逗号都替换成全角的
	for i, v := range req.RightAnswer {
		req.RightAnswer[i] = strings.Replace(v, ",", "，", -1)
	}
	for i, v := range req.WrongAnswer {
		req.WrongAnswer[i] = strings.Replace(v, ",", "，", -1)
	}

	question, rightAnswer, wrongAnswer := processByAddType(req.AddType, req.Question, req.RightAnswer, req.WrongAnswer)

	result, err := s_DB.Exec("insert INTO ctb_choice_question(type,question,right_answer,wrong_answer) values(1,?,?,?)",
		question, rightAnswer, wrongAnswer)
	checkf(err, "insert question failed")
	lastInsertID, err := result.LastInsertId()
	checkf(err, "get lastInsertID failed")
	fmt.Printf("Insert choice question success, id = %d, user = %d\n", lastInsertID, user_id)
	_, err = s_DB.Exec("insert INTO ctb_answer_record(question_id,user_id,rest_cnt,next_time) values(?,?,?,now())", lastInsertID, user_id, req.RestCount)
	checkf(err, "insert record failed")
}

func processByAddType(addType, oriQuestion string, oriRightAnswer, oriWrongAnswer []string) (question, rightAnswer, wrongAnswer string) {
	rightAnswer = strings.Join(oriRightAnswer, ",")
	wrongAnswer = strings.Join(oriWrongAnswer, ",")

	switch addType {
	case "wrong_character":
		question = ReplaceAllPinyin(oriQuestion, "??(", ")")
	case "wrong_pinyin":
		question = "注音：" + oriQuestion
		rightAnswer = ReplaceAllPinyin(rightAnswer, "", "")
		wrongAnswer = ReplaceAllPinyin(wrongAnswer, "", "")
	case "text":
	default:
		panic("unknown add_type: " + addType)
	}
	return
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
	user_id := getUserId(r)
	var rsp struct {
		Id          int      `json:"id"`
		Type        int      `json:"type"`
		Question    string   `json:"question"`
		Choices     []string `json:"choices"`
		RightAnswer string
		WrongAnswer string
	}

	row := s_DB.QueryRow("select question_id from ctb_answer_record where user_id = ? and now() > next_time and rest_cnt > 0 order by next_time limit 1", user_id)
	if err := row.Scan(&rsp.Id); err != nil {
		return 0, nil
	}
	row = s_DB.QueryRow("select count(0) from ctb_answer_record where user_id = ? and now() > next_time and rest_cnt > 0", user_id)
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
	user_id := getUserId(r)
	result := []interface{}{}

	rows, err := s_DB.Query(`select question_id, question, rest_cnt, right_cnt, wrong_cnt from ctb_answer_record, ctb_choice_question
		where id = question_id and user_id = ? and rest_cnt > 0 limit 1000`, user_id)
	if rows != nil {
		defer rows.Close()
	}
	if err != nil {
		panic("query learning question list from db failed")
	}
	for rows.Next() {
		var id, cnt, right, wrong int
		var question string
		err = rows.Scan(&id, &question, &cnt, &right, &wrong)
		if err != nil {
			panic(fmt.Sprintf("scan failed: %v", err))
		}
		result = append(result, &struct {
			Id         int    `json:"id"`
			Question   string `json:"question"`
			RestCount  int    `json:"rest_count"`
			RightCount int    `json:"right_count"`
			WrongCount int    `json:"wrong_count"`
		}{id, question, cnt, right, wrong})
	}
	return result
}

func submitAnswer(w http.ResponseWriter, r *http.Request) {
	defer errRecover4Rest(w)

	id := r.FormValue("id")
	user_id := getUserId(r)
	chosen := strings.Split(r.FormValue("chosen"), ",")
	sort.Strings(chosen)
	row := s_DB.QueryRow("select right_answer from ctb_choice_question where id = ?", id)
	var correctAnswer string
	checkf(row.Scan(&correctAnswer), "question not exist")
	correct := strings.Split(correctAnswer, ",")
	sort.Strings(correct)
	var rsp struct {
		Result  string `json:"result"`
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
	rsp.Result = "ok"
	rsp.Detail = fmt.Sprintf("答案：%s", correctAnswer)
	b, err := json.Marshal(&rsp)
	checkf(err, "marshal json failed")
	w.Write(b)

	// 另起协程让页面响应更快
	go func() {
		if rsp.Correct {
			// 按记忆曲线更新下次学习的时间
			_, err := s_DB.Exec(`update ctb_answer_record set rest_cnt = rest_cnt - 1, next_time = date_add(now(), interval (case rest_cnt
					when 1 then 14*24*60
					when 2 then 5*24*60
					when 3 then 2*24*60
					when 4 then 24*60
					when 5 then 12*60
					when 6 then 10*60
					when 7 then 3*60
					when 8 then 60
					when 9 then 20
					else 5 end
				) minute), right_cnt = right_cnt + 1 where question_id = ? and user_id = ?`, id, user_id)
			if err != nil {
				fmt.Printf("update answer record failed, %v\n", err)
			}
		} else {
			_, err := s_DB.Exec("update ctb_answer_record set rest_cnt = rest_cnt + 5, next_time = now(), wrong_cnt = wrong_cnt + 1 where question_id = ? and user_id = ?",
				id, user_id)
			if err != nil {
				fmt.Printf("update answer record failed, %v\n", err)
			}
		}
	}()
}

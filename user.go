package main

import (
	"fmt"
	"net/http"
	"regexp"
)

func handleRegisterOrLogin(w http.ResponseWriter, r *http.Request) {
	defer errRecover4Rest(w)

	user := r.FormValue("user")
	user_id := 0
	row := s_DB.QueryRow("select id from ctb_user where user = ?", user)
	err := row.Scan(&user_id)
	if err == nil && user_id > 0 {
		w.Write([]byte(`{"result":"ok"}`))
		return
	}

	match, err := regexp.MatchString(`[a-zA-Z0-9]+`, user)
	if !match || err != nil {
		fmt.Printf("Invalid user name %s, err = %v\n", user, err)
	}

	result, err := s_DB.Exec("insert INTO ctb_user(user) values(?)", user)
	checkf(err, "add user failed")
	lastInsertID, err := result.LastInsertId()
	checkf(err, "add user get id failed")

	w.Write([]byte(`{"result":"ok"}`))
	fmt.Printf("Add new user %s, id %d\n", user, lastInsertID)
}

func getUserId(r *http.Request) int {
	user := r.FormValue("user")
	if user == "" {
		panic("user is empty")
	}

	user_id := 0
	row := s_DB.QueryRow("select id from ctb_user where user = ?", user)
	checkf(row.Scan(&user_id), "get user from db failed")

	return user_id
}

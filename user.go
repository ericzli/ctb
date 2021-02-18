package main

import (
	"fmt"
	"net/http"
	"regexp"
)

func handleRegisterOrLogin(w http.ResponseWriter, r *http.Request) {
	defer errRecover4Rest(w)

	user := r.FormValue("user")
	match, err := regexp.MatchString(`[a-zA-Z0-9]+`, user)
	if !match || err != nil {
		fmt.Printf("Invalid user name %s, err = %v\n", user, err)
	}

	result, err := s_DB.Exec("insert INTO ctb_user(user) select ? where not exists (select user from ctb_user where user = ?)",
		user, user)
	checkf(err, "add user failed")
	lastInsertID, err := result.LastInsertId()
	if err == nil {
		fmt.Printf("Add new user %s, id %d\n", user, lastInsertID)
	}

	w.Write([]byte(`{"result":"ok"}`))
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

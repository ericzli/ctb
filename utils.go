package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func checkf(err error, prefix string) {
	if err != nil {
		panic(fmt.Sprintf("%s: %v", prefix, err))
	}
}

func errRecover4Rest(w http.ResponseWriter) {
	err := recover()
	var rsp struct {
		Result string `json:"result"`
	}
	if err != nil {
		debug.PrintStack()
		rsp.Result = fmt.Sprintf("server failed: %v", err)
		encoder := json.NewEncoder(w)
		encoder.Encode(&rsp)
	}
}

package main

import "fmt"

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

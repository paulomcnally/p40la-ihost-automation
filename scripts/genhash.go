package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := "test1234"
	if len(os.Args) > 1 && os.Args[1] != "" {
		password = os.Args[1]
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error generando hash:", err)
		os.Exit(1)
	}
	fmt.Println(string(h))
}
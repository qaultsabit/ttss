package main

import (
	"fmt"
	"os"
)

const (
	address  string = "localhost:2222"
	user     string = "tsabit"
	password string = "password"
	logdir   string = "/home/tsabit/logs"
	DBConn   string = "oracle://user:password@host:1521/service_name"
	query    string = "SELECT * FROM DOC WHERE srn = :1"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: ttss <srn> <dir>")
		return
	}
	srn, dir := os.Args[1], os.Args[2]

	files, err := getlogs(srn, dir)
	if err != nil {
		rollBack(dir)
		fmt.Println(err)
		return
	}

	doc, err := getDoc(srn, dir)
	if err != nil {
		rollBack(dir)
		fmt.Println(err)
		return
	}

	if len(files) < 4 {
		rollBack(dir)
		return
	}

	for _, file := range files {
		fmt.Println(file)
	}
	fmt.Println(doc)
}

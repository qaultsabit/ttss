package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	address  string = "localhost:2222"
	user     string = "tsabit"
	password string = "password"
	logdir   string = "/home/tsabit/logs"
	DBConn   string = "oracle://user:password@host:1521/service_name"
	query    string = "SELECT * FROM DOC WHERE SOURCE_REG_NUM = :1"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: ts <srn> <dir>")
		return
	}
	srn, dest := os.Args[1], os.Args[2]

	files, err := getLogs(srn, dest)
	if err != nil {
		fmt.Println(err)
		return
	}

	// doc, err := getDoc(srn, dest)
	// if err != nil {
	//     fmt.Println(err)
	//     return
	// }
	// files = append(files, doc)

	if len(files) < 4 {
		rollBack(dest)
		return
	}

	for _, file := range files {
		fmt.Println(file)
	}
}

func rollBack(dest string) {
	files, err := filepath.Glob(filepath.Join(dest, "*"))
	if err != nil {
		fmt.Println("failed")
		return
	}

	for _, file := range files {
		os.Remove(file)
	}
}

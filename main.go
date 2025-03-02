package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LogType int

const (
	EXT LogType = iota
	ATM
	BASE
	BOOTSTRAP
)

var logKeywords = map[LogType]string{
	EXT:       "ext",
	ATM:       "atm",
	BASE:      "base",
	BOOTSTRAP: "bootstrap",
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: ttss <srn> <dest_path>")
		return
	}

	srn, destPath := os.Args[1], os.Args[2]

	if err := os.MkdirAll(destPath, 0777); err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		return
	}

	files, err := os.ReadDir(".")
	if err != nil {
		fmt.Printf("Error reading directory: %v\n", err)
		return
	}

	logs := findLatestLogs(files, srn)
	if len(logs) == 0 {
		fmt.Println("No matching log files found.")
		return
	}

	for _, file := range logs {
		src := file
		dst := filepath.Join(destPath, file)

		if err := copyFile(src, dst); err != nil {
			fmt.Printf("Error copying file %s: %v\n", src, err)
			return
		}
		fmt.Printf("Copied %s to %s\n", src, dst)
	}

	fmt.Println("Done")
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func containsSRN(filename, srn string) bool {
	file, err := os.Open(filename)
	if err != nil {
		return false
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if strings.Contains(line, srn) {
			return true
		}
		if err != nil {
			break
		}
	}

	return false
}

func findLatestLogs(files []os.DirEntry, srn string) map[LogType]string {
	logs := make(map[LogType]string)

	for logType, keyword := range logKeywords {
		var latestFile string
		var latestTime time.Time

		for _, file := range files {
			if file.IsDir() || !strings.Contains(file.Name(), keyword) {
				continue
			}

			if logType == EXT && !containsSRN(file.Name(), srn) {
				continue
			}

			info, err := file.Info()
			if err != nil {
				continue
			}

			if info.ModTime().After(latestTime) {
				latestTime = info.ModTime()
				latestFile = file.Name()
			}
		}

		if latestFile != "" {
			logs[logType] = latestFile
		}
	}

	return logs
}

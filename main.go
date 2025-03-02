package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	_ "github.com/sijms/go-ora/v2"
	excelize "github.com/xuri/excelize/v2"
	"golang.org/x/crypto/ssh"
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

	err := getlogs(srn, dir)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = getDoc(srn, dir)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func getlogs(srn, dir string) error {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("error creating directory: %v\n", err)
	}

	client, err := connectSFTP(address, user, password)
	if err != nil {
		return fmt.Errorf("error connecting to server: %v\n", err)
	}
	defer client.Close()

	logs, err := client.ReadDir(logdir)
	if err != nil {
		return fmt.Errorf("error getting logs: %v\n", err)
	}

	keywords := [4]string{"ext", "atm", "base", "bootstrap"}
	for _, keyword := range keywords {
		if keyword == "ext" {
			extLogs, err := getExtLog(client, logs, srn)
			if err != nil {
				fmt.Printf("error getting ext logs: %v\n", err)
				continue
			}

			for _, log := range extLogs {
				if err := downloadLog(client, path.Join(logdir, log), filepath.Join(dir, log)); err != nil {
					return fmt.Errorf("error downloading log: %v\n", err)
				}

				fmt.Println(log)
			}
		} else {
			log, err := getLatestLog(logs, keyword)
			if err != nil {
				return fmt.Errorf("error getting log: %v\n", err)
			}

			if err := downloadLog(client, path.Join(logdir, log), filepath.Join(dir, log)); err != nil {
				return fmt.Errorf("error downloading log: %v\n", err)
			}

			fmt.Println(log)
		}
	}

	return nil
}

func getDoc(srn, dir string) error {
	db, err := sql.Open("oracle", DBConn)
	if err != nil {
		return fmt.Errorf("failed to connect database: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(query, srn)
	if err != nil {
		return fmt.Errorf("failed to execute query: %v", err)
	}
	defer rows.Close()

	f := excelize.NewFile()
	sheetName := "Sheet1"
	f.SetSheetName(f.GetSheetName(0), sheetName)

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %v", err)
	}

	for i, col := range columns {
		cell := fmt.Sprintf("%s1", string(rune('A'+i)))
		f.SetCellValue(sheetName, cell, col)
	}

	rowIndex := 2
	for rows.Next() {
		values := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}

		if err := rows.Scan(dest...); err != nil {
			return fmt.Errorf("failed to scan row: %v", err)
		}

		for i, val := range values {
			cell := fmt.Sprintf("%s%d", string(rune('A'+i)), rowIndex)
			if err := f.SetCellValue(sheetName, cell, fmt.Sprintf("%v", val)); err != nil {
				return fmt.Errorf("failed to set cell value: %v", err)
			}
		}
		rowIndex++
	}

	filePath := filepath.Join(dir, "doc.xlsx")
	if err := f.SaveAs(filePath); err != nil {
		return fmt.Errorf("failed to save Excel file: %v", err)
	}

	return nil
}

func connectSFTP(addr, user, password string) (*sftp.Client, error) {
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         8 * time.Second,
	}

	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return client, nil
}

func getLatestLog(logs []os.FileInfo, keyword string) (string, error) {
	var latestLog string
	var latestTime time.Time
	found := false

	for _, log := range logs {
		if log.IsDir() || !strings.HasPrefix(log.Name(), keyword) {
			continue
		}
		if log.ModTime().After(latestTime) {
			latestTime = log.ModTime()
			latestLog = log.Name()
			found = true
		}
	}

	if !found {
		return "", fmt.Errorf("%s logs not found", keyword)
	}

	return latestLog, nil
}

func getExtLog(client *sftp.Client, logs []os.FileInfo, srn string) ([]string, error) {
	var result []string
	for _, log := range logs {
		if log.IsDir() || !strings.HasPrefix(log.Name(), "ext") {
			continue
		}

		logPath := path.Join(logdir, log.Name())
		file, err := client.Open(logPath)
		if err != nil {
			return result, fmt.Errorf("error opening file: %v", err)
		}

		scanner := bufio.NewScanner(file)
		found := false
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), srn) {
				result = append(result, log.Name())
				found = true
				break
			}
		}
		file.Close()

		if err := scanner.Err(); err != nil {
			return result, err
		}

		if found {
			continue
		}
	}

	if len(result) == 0 {
		return result, fmt.Errorf("ext logs not found")
	}

	return result, nil
}

func downloadLog(client *sftp.Client, remotePath, localPath string) error {
	srcFile, err := client.Open(remotePath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

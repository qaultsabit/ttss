package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	address  string = "localhost:2222"
	user     string = "tsabit"
	password string = "password"
	logdir   string = "/home/tsabit/logs"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: ttss <srn> <dir>")
		return
	}
	srn, dir := os.Args[1], os.Args[2]

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		fmt.Printf("error creating directory: %v\n", err)
		return
	}

	client, err := connectSFTP(address, user, password)
	if err != nil {
		fmt.Printf("error connecting to server: %v\n", err)
		return
	}
	defer client.Close()

	keywords := [4]string{"ext", "atm", "base", "bootstrap"}
	for _, keyword := range keywords {
		if keyword == "ext" {
			logs, err := getExtLog(client, logdir, srn)
			if err != nil {
				fmt.Printf("error getting ext logs: %v\n", err)
				return
			} else if len(logs) == 0 {
				fmt.Printf("%s log not found\n", keyword)
				return
			}

			for _, log := range logs {
				if err := downloadLog(client, path.Join(logdir, log), filepath.Join(dir, log)); err != nil {
					fmt.Printf("error downloading log: %v\n", err)
					return
				}
				fmt.Println(log)
			}
		} else {
			log, err := getLatestLog(client, logdir, keyword)
			if err != nil {
				fmt.Printf("error getting %s logs: %v\n", keyword, err)
				return
			} else if log == "" {
				fmt.Printf("%s log not found\n", keyword)
				return
			}

			if err := downloadLog(client, path.Join(logdir, log), filepath.Join(dir, log)); err != nil {
				fmt.Printf("error downloading log: %v\n", err)
				return
			}

			fmt.Println(log)
		}
	}
}

func connectSFTP(addr, user, password string) (*sftp.Client, error) {
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
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

func getLatestLog(client *sftp.Client, dir, keyword string) (string, error) {
	logs, err := client.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var latestLog string
	var latestTime time.Time

	for _, log := range logs {
		if log.IsDir() || !strings.HasPrefix(log.Name(), keyword) {
			continue
		}
		if log.ModTime().After(latestTime) {
			latestTime = log.ModTime()
			latestLog = log.Name()
		}
	}
	return latestLog, nil
}

func getExtLog(client *sftp.Client, logdir, srn string) ([]string, error) {
	var result []string
	logs, err := client.ReadDir(logdir)
	if err != nil {
		return result, err
	}

	for _, log := range logs {
		if log.IsDir() || !strings.HasPrefix(log.Name(), "ext") {
			continue
		}

		file, err := client.Open(path.Join(logdir, log.Name()))
		if err != nil {
			return result, err
		}
		defer file.Close()

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

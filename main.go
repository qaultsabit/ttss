package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
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

	logs, err := getLogs(client, logdir)
	if err != nil {
		fmt.Printf("error reading logs directory: %v\n", err)
		return
	}

	keywords := [4]string{"ext", "atm", "base", "bootstrap"}
	for _, keyword := range keywords {
		if keyword == "ext" {
			extLogs, err := getExtLog(client, logs, srn)
			if err != nil {
				fmt.Printf("error getting ext logs: %v\n", err)
				continue
			} else if len(extLogs) == 0 {
				fmt.Println("ext logs not found")
				continue
			}

			var wg sync.WaitGroup
			for _, log := range extLogs {
				wg.Add(1)
				go func(log string) {
					defer wg.Done()
					if err := downloadLog(client, path.Join(logdir, log), filepath.Join(dir, log)); err != nil {
						fmt.Printf("error downloading log: %v\n", err)
						return
					}
					fmt.Println(log)
				}(log)
			}
			wg.Wait()
		} else {
			log, err := getLatestLog(logs, keyword)
			if err != nil {
				fmt.Printf("error getting %s logs: %v\n", keyword, err)
				continue
			} else if log == "" {
				fmt.Printf("%s log not found\n", keyword)
				continue
			}

			if err := downloadLog(client, path.Join(logdir, log), filepath.Join(dir, log)); err != nil {
				fmt.Printf("error downloading log: %v\n", err)
				continue
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

func getLogs(client *sftp.Client, dir string) ([]os.FileInfo, error) {
	return client.ReadDir(dir)
}

func getLatestLog(logs []os.FileInfo, keyword string) (string, error) {
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

func getExtLog(client *sftp.Client, logs []os.FileInfo, srn string) ([]string, error) {
	var result []string
	for _, log := range logs {
		if log.IsDir() || !strings.HasPrefix(log.Name(), "ext") {
			continue
		}

		file, err := client.Open(path.Join(logdir, log.Name()))
		if err != nil {
			return result, err
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

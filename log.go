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

func getlogs(srn, dir string) ([]string, error) {
	var result []string
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return result, fmt.Errorf("error creating directory: %v", err)
	}

	client, err := connectSFTP(address, user, password)
	if err != nil {
		return result, fmt.Errorf("error connecting to server: %v", err)
	}
	defer client.Close()

	logs, err := client.ReadDir(logdir)
	if err != nil {
		return result, fmt.Errorf("error getting logs: %v", err)
	}

	keywords := [4]string{"ext", "atm", "base", "bootstrap"}
	var extModTime time.Time
	var atmModTime time.Time

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, keyword := range keywords {
		wg.Add(1)
		go func(keyword string) {
			defer wg.Done()
			if keyword == "ext" {
				extLogs, time, err := getExtLog(client, logs, srn)
				mu.Lock()
				extModTime = time
				mu.Unlock()
				if err != nil {
					fmt.Printf("error getting ext logs: %v\n", err)
					return
				}

				for _, log := range extLogs {
					if err := downloadLog(client, path.Join(logdir, log), filepath.Join(dir, log)); err != nil {
						fmt.Printf("error downloading log: %v\n", err)
						return
					}

					mu.Lock()
					result = append(result, log)
					mu.Unlock()
				}
			} else {
				log, time, err := getLatestLog(logs, keyword)
				if keyword == "atm" {
					mu.Lock()
					atmModTime = time
					mu.Unlock()
				}
				if err != nil {
					fmt.Printf("error getting log: %v\n", err)
					return
				}

				if err := downloadLog(client, path.Join(logdir, log), filepath.Join(dir, log)); err != nil {
					fmt.Printf("error downloading log: %v\n", err)
					return
				}

				mu.Lock()
				result = append(result, log)
				mu.Unlock()
			}
		}(keyword)
	}

	wg.Wait()

	if extModTime.Sub(atmModTime).Minutes() > 2 || atmModTime.Sub(extModTime).Minutes() > 2 {
		for _, log := range result {
			os.Remove(filepath.Join(dir, log))
		}
		return result, fmt.Errorf("production mode")
	}

	return result, nil
}

func connectSFTP(addr, user, password string) (*sftp.Client, error) {
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
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

func getLatestLog(logs []os.FileInfo, keyword string) (string, time.Time, error) {
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
		return "", latestTime, fmt.Errorf("%s logs not found", keyword)
	}

	return latestLog, latestTime, nil
}

func getExtLog(client *sftp.Client, logs []os.FileInfo, srn string) ([]string, time.Time, error) {
	var result []string
	var modTime time.Time
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, log := range logs {
		if log.IsDir() || !strings.HasPrefix(log.Name(), "ext") {
			continue
		}

		wg.Add(1)
		go func(log os.FileInfo) {
			defer wg.Done()
			logPath := path.Join(logdir, log.Name())
			file, err := client.Open(logPath)
			if err != nil {
				fmt.Printf("error opening file: %v\n", err)
				return
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				if strings.Contains(scanner.Text(), srn) {
					mu.Lock()
					result = append(result, log.Name())
					modTime = log.ModTime()
					mu.Unlock()
					break
				}
			}

			if err := scanner.Err(); err != nil {
				fmt.Printf("error scanning file: %v\n", err)
				return
			}
		}(log)
	}

	wg.Wait()

	if len(result) == 0 {
		return result, modTime, fmt.Errorf("ext logs not found")
	}

	return result, modTime, nil
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

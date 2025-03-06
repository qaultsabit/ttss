package main

import (
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func getLogs(srn, dest string) ([]string, error) {
	if err := os.MkdirAll(dest, os.ModePerm); err != nil {
		return nil, err
	}

	sshClient, sftpClient, err := connect(address, user, password)
	if err != nil {
		return nil, err
	}
	defer sshClient.Close()
	defer sftpClient.Close()

	extLogs, err := getExtLogs(sshClient, srn)
	if err != nil {
		return nil, err
	}

	anotherLogs, err := getAnotherLogs(sshClient)
	if err != nil {
		return nil, err
	}

	if isProd, err := isProdMode(sftpClient, extLogs[0], anotherLogs[0]); err != nil {
		return nil, err
	} else if isProd {
		return nil, fmt.Errorf("production mode")
	}

	return downloadLogs(sftpClient, append(extLogs, anotherLogs...), dest)
}

func connect(addr, user, password string) (*ssh.Client, *sftp.Client, error) {
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to dial SSH: %w", err)
	}

	sftpClient, err := sftp.NewClient(conn)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

	return conn, sftpClient, nil
}

func getAnotherLogs(sshClient *ssh.Client) ([]string, error) {
	keywords := []string{"atm", "base", "bootstrap"}
	logs := make([]string, len(keywords))

	for i, keyword := range keywords {
		log, err := getLatestLog(sshClient, keyword)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest log for keyword %s: %w", keyword, err)
		}
		logs[i] = log
	}

	return logs, nil
}

func getLatestLog(sshClient *ssh.Client, keyword string) (string, error) {
	cmd := fmt.Sprintf("ls -t %s/%s* | head -n 1 | xargs -I {} basename {}", logdir, keyword)
	output, err := runCommand(sshClient, cmd)
	if err != nil || output == "" {
		return "", fmt.Errorf("%s log not found: %w", keyword, err)
	}
	return strings.TrimSpace(output), nil
}

func getExtLogs(sshClient *ssh.Client, srn string) ([]string, error) {
	cmd := fmt.Sprintf("grep -rl %s --include=ext* %s | xargs -I {} basename {}", srn, logdir)
	output, err := runCommand(sshClient, cmd)
	if err != nil || output == "" {
		return nil, fmt.Errorf("ext logs not found: %w", err)
	}
	return strings.Split(strings.TrimSpace(output), "\n"), nil
}

func runCommand(sshClient *ssh.Client, command string) (string, error) {
	session, err := sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return "", fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

func isProdMode(sftpClient *sftp.Client, extLog, atmLog string) (bool, error) {
	extFileInfo, err := sftpClient.Stat(fmt.Sprintf("%s/%s", logdir, extLog))
	if err != nil {
		return false, err
	}
	extModTime := extFileInfo.ModTime()

	atmFileInfo, err := sftpClient.Stat(fmt.Sprintf("%s/%s", logdir, atmLog))
	if err != nil {
		return false, err
	}
	atmModTime := atmFileInfo.ModTime()

	if math.Abs(atmModTime.Sub(extModTime).Minutes()) > 3 {
		return true, fmt.Errorf("production mode")
	}

	return false, nil
}

func downloadLogs(sftpClient *sftp.Client, logs []string, dest string) ([]string, error) {
	var wg sync.WaitGroup
	errChan := make(chan error, len(logs))
	for _, log := range logs {
		wg.Add(1)
		go func(log string) {
			defer wg.Done()
			remotePath := fmt.Sprintf("%s/%s", logdir, log)
			localPath := fmt.Sprintf("%s/%s", dest, log)
			if err := downloadFile(sftpClient, remotePath, localPath); err != nil {
				errChan <- fmt.Errorf("failed downloading file %s: %w", log, err)
			}
		}(log)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}

	return logs, nil
}

func downloadFile(sftpClient *sftp.Client, remotePath, localPath string) error {
	srcFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer dstFile.Close()

	if _, err := srcFile.WriteTo(dstFile); err != nil {
		return fmt.Errorf("failed to write to local file: %w", err)
	}

	return nil
}

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func main() {
	gitUrl := ""
	username := ""
	if len(os.Args) != 3 {
		fmt.Println("Usage: git-chat <git-repo-url> <username>")
		return
	}
	gitUrl = os.Args[1]
	username = os.Args[2]

	tempDir, err := os.MkdirTemp("", "git-clone-")
	if err != nil {
		fmt.Printf("Failed to create temp directory: %v\n", err)
		return
	}
	defer os.RemoveAll(tempDir)

	cmd := exec.Command("git", "clone", gitUrl, tempDir)
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Failed to clone repository: %v\n", err)
		return
	}

	fmt.Printf("Repository cloned to: %s\n", tempDir)

	ticker := time.NewTicker(3 * time.Second)
	done := make(chan bool)
	messages := make([]string, 0, 20)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				cmd := exec.Command("git", "-C", tempDir, "fetch")
				err := cmd.Run()
				if err != nil {
					fmt.Printf("Failed to fetch: %v\n", err)
				} else {
					logCmd := exec.Command("git", "-C", tempDir, "log", "origin/main", "--oneline", "-n", "20")
					output, err := logCmd.Output()
					if err != nil {
						fmt.Printf("Failed to get git log: %v\n", err)
					} else {
						newMessages := make([]string, 0, 20)
						scanner := bufio.NewScanner(strings.NewReader(string(output)))
						for scanner.Scan() {
							newMessage := scanner.Text()
							re := regexp.MustCompile(`.+ \[(.+)]: (.+)`)
							matches := re.FindStringSubmatch(newMessage)
							if len(matches) == 3 {
								parsedMessage := fmt.Sprintf("[%s]: %s", matches[1], matches[2])
								if !contains(messages, parsedMessage) {
									newMessages = append([]string{parsedMessage}, newMessages...)
								}
							}
						}
						if len(newMessages) > 0 {
							messages = append(messages, newMessages...)
							if len(messages) > 20 {
								messages = messages[len(messages)-20:]
							}
							for _, message := range newMessages {
								fmt.Println(message)
							}
						}
					}
				}
			}
		}
	}()

	fmt.Println("Type a message and press Enter to send. Type 'exit' to stop...")

	inputChan := make(chan string)
	go readInput(inputChan)

	for {
		select {
		case input := <-inputChan:
			if input == "exit" {
				ticker.Stop()
				done <- true
				return
			}
			sendMessage(tempDir, username, input)
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func readInput(inputChan chan<- string) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		inputChan <- scanner.Text()
	}
}

func sendMessage(tempDir string, user string, message string) {
	fmt.Printf("[%s]: %s\n", user, message)

	filename := user + ".txt"
	filePath := filepath.Join(tempDir, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		_, err := os.Create(filePath)
		if err != nil {
			fmt.Printf("Error creating file: %v\n", err)
			return
		}
	}

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer file.Close()

	_, err = file.WriteString(fmt.Sprintf("[%s]: %s\n", user, message))
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
		return
	}

	cmd := exec.Command("git", "-C", tempDir, "add", filePath)
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Error adding file to git: %v\n", err.Error())
		return
	}

	cmd = exec.Command("git", "-C", tempDir, "commit", "-m", fmt.Sprintf("[%s]: %s", user, message))
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Error committing changes: %v\n", err)
		return
	}

	cmd = exec.Command("git", "-C", tempDir, "push", "--force")
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Error pushing changes: %v\n", err)
		return
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	for {
		fmt.Fprint(os.Stdout, "$ ")
		command, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input:", err)
			os.Exit(1)
		}

		command = command[:len(command)-1]
		if strings.Contains(command, "exit") {
			exitCode, _ := strconv.Atoi(command[4:])
			os.Exit(exitCode)
		}

		if strings.Contains(command, "echo") {
			fmt.Println(strings.TrimLeft(command[4:], " "))
			continue
		}

		fmt.Println(command + ": command not found")
	}
}

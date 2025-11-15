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

		fmt.Println(command + ": command not found")
	}
}

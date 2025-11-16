package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type handleCmd func(commandParts []string)

var builtins map[string]handleCmd

func init() {
	builtins = map[string]handleCmd{
		"exit": handleExitCmd,
		"echo": handleEchoCmd,
		"type": handleTypeCmd,
	}
}

func main() {
	for {
		fmt.Fprint(os.Stdout, "$ ")
		command, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input:", err)
			os.Exit(1)
		}

		command = command[:len(command)-1]
		parts := strings.Fields(command)
		if len(parts) == 0 {
			continue
		}

		cmdName := parts[0]
		if execute, ok := builtins[cmdName]; ok {
			execute(parts)
			continue
		}

		if findExecutable(parts) {
			continue
		}

		fmt.Printf("%s: command not found\n", command)
	}
}

func handleExitCmd(commandParts []string) {
	exitCode := 0
	if len(commandParts) > 1 {
		exitCode, _ = strconv.Atoi(commandParts[1])
	}

	os.Exit(exitCode)
}

func handleEchoCmd(commandParts []string) {
	if len(commandParts) > 1 {
		fmt.Println(strings.Join(commandParts[1:], " "))
	}
}

func handleTypeCmd(commandParts []string) {
	if len(commandParts) < 2 {
		return
	}

	arg := commandParts[1]
	if _, ok := builtins[arg]; ok {
		fmt.Printf("%s is a shell builtin\n", arg)
		return
	}

	envPath := os.Getenv("PATH")
	directories := strings.Split(envPath, ":")

	for _, dir := range directories {
		abs := filepath.Join(filepath.Clean(dir), arg)
		stat, err := os.Stat(abs)
		if err == nil && stat.Mode()&0111 != 0 {
			fmt.Printf("%s is %s\n", arg, abs)
			return
		}
	}

	fmt.Printf("%s: not found\n", arg)
}

func findExecutable(commandParts []string) bool {
	cmdName := commandParts[0]
	envPath := os.Getenv("PATH")
	directories := strings.Split(envPath, ":")

	for _, dir := range directories {
		abs := filepath.Join(filepath.Clean(dir), cmdName)
		stat, err := os.Stat(abs)
		if err == nil && stat.Mode()&0111 != 0 {
			var args []string
			if len(commandParts) > 1 {
				args = commandParts[1:]
			}

			executable := exec.Command(cmdName, args...)
			executable.Stdin = os.Stdin
			executable.Stdout = os.Stdout
			executable.Stderr = os.Stderr
			executable.Run()

			return true
		}
	}

	return false
}

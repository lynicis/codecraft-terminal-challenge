package main

import (
	"bufio"
	"fmt"
	"os"
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

		fmt.Println(fmt.Printf("%s: command not found", command))
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

	cmdName := commandParts[1]
	if _, ok := builtins[cmdName]; ok {
		fmt.Println(fmt.Printf("%s is a shell builtin", cmdName))
		return
	}

	pathVar := os.Getenv("PATH")
	directories := strings.Split(pathVar, ":")

	for _, dir := range directories {
		fullPath := filepath.Join(dir, cmdName)
		stat, err := os.Stat(fullPath)
		if err == nil {
			isExecutable := !stat.IsDir() && (stat.Mode()&0111 != 0)
			if isExecutable {
				fmt.Printf("%v is %v", cmdName, fullPath)
				return
			}
		}
	}

	/* var foundPath string
	filepath.Walk("/", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		fileName := filepath.Base(path)
		if !info.IsDir() && fileName == cmdName && info.Mode()&0111 != 0 {
			foundPath = path
			return io.EOF
		}

		return nil
	})

	if foundPath != "" {
		fmt.Println(fmt.Printf("%s is %s", cmdName, foundPath))
	} */
}

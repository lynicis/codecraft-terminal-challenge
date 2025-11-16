package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type handleCmd func(commandParts []string)

var builtins map[string]handleCmd

var (
	pathDirs     []string
	pathDirsOnce sync.Once
)

func init() {
	builtins = map[string]handleCmd{
		"exit": handleExitCmd,
		"echo": handleEchoCmd,
		"type": handleTypeCmd,
		"pwd":  handlePwdCmd,
		"cd":   handleCdCmd,
	}
}

func main() {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Fprint(os.Stdout, "$ ")
		command, err := reader.ReadString('\n')
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

		_, ok := findProgramInEnv(cmdName)
		if ok {
			runExecutable(cmdName, parts[1:])
			continue
		}

		fmt.Printf("%s: command not found\n", command)
	}
}

func handleExitCmd(commandParts []string) {
	var exitCode int
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
		fmt.Printf("%s is a shell builtin\n", cmdName)
		return
	}

	abs, ok := findProgramInEnv(cmdName)
	if ok {
		fmt.Printf("%s is %s\n", cmdName, abs)
		return
	}

	fmt.Printf("%s: not found\n", cmdName)
}

func handlePwdCmd(commandParts []string) {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s\n", cwd)
}

func handleCdCmd(commandParts []string) {
	targetPath := commandParts[1]
	if targetPath == "~" {
		targetPath = os.Getenv("HOME")
	}

	if err := os.Chdir(targetPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Printf("cd: %s: No such file or directory\n", targetPath)
		}
	}
}

func runExecutable(cmdName string, args []string) {
	executable := exec.Command(cmdName, args...)
	executable.Stdin = os.Stdin
	executable.Stdout = os.Stdout
	executable.Stderr = os.Stderr
	executable.Run()
}

func getPathDirs() []string {
	pathDirsOnce.Do(func() {
		envPath := os.Getenv("PATH")
		pathDirs = strings.Split(envPath, ":")
	})

	return pathDirs
}

func findProgramInEnv(cmdName string) (string, bool) {
	directories := getPathDirs()

	for _, dir := range directories {
		abs := filepath.Join(filepath.Clean(dir), cmdName)
		stat, err := os.Stat(abs)
		if err == nil && stat.Mode()&0111 != 0 {
			return abs, true
		}
	}

	return "", false
}

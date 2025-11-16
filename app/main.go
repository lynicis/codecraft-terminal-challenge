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
		parsed := parseCommand(command)
		if len(parsed.Parts) == 0 {
			continue
		}

		cmdName := parsed.Parts[0]
		if execute, ok := builtins[cmdName]; ok {
			applyRedirectionsToBuiltin(execute, parsed.Parts, parsed.Redirections)
			continue
		}

		_, ok := findProgramInEnv(cmdName)
		if ok {
			runExecutable(cmdName, parsed.Parts[1:], parsed.Redirections)
			continue
		}

		fmt.Printf("%s: command not found\n", cmdName)
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
		args := commandParts[1:]
		fmt.Println(strings.Join(args, " "))
	} else {
		fmt.Println()
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
	var targetPath string
	if len(commandParts) > 1 {
		targetPath = commandParts[1]
		if targetPath == "~" {
			targetPath = os.Getenv("HOME")
		}
	}

	if err := os.Chdir(targetPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Printf("cd: %s: No such file or directory\n", targetPath)
		}
	}
}

func applyRedirectionsToBuiltin(execute handleCmd, commandParts []string, redirections []Redirection) {
	if len(redirections) == 0 {
		execute(commandParts)
		return
	}

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	originalStdin := os.Stdin

	var stdoutFile, stderrFile, stdinFile *os.File
	var err error

	for _, redir := range redirections {
		switch redir.Type {
		case ">", "1>":
			stdoutFile, err = os.Create(redir.Filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating file %s: %v\n", redir.Filename, err)
				return
			}
			os.Stdout = stdoutFile
		case ">>", "1>>":
			stdoutFile, err = os.OpenFile(redir.Filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening file %s: %v\n", redir.Filename, err)
				return
			}
			os.Stdout = stdoutFile
		case "<":
			stdinFile, err = os.Open(redir.Filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening file %s: %v\n", redir.Filename, err)
				return
			}
			os.Stdin = stdinFile
		case "2>":
			stderrFile, err = os.Create(redir.Filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating file %s: %v\n", redir.Filename, err)
				return
			}
			os.Stderr = stderrFile
		case "2>>":
			stderrFile, err = os.OpenFile(redir.Filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening file %s: %v\n", redir.Filename, err)
				return
			}
			os.Stderr = stderrFile
		case "&>", ">&":
			stdoutFile, err = os.Create(redir.Filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating file %s: %v\n", redir.Filename, err)
				return
			}
			os.Stdout = stdoutFile
			os.Stderr = stdoutFile
		}
	}

	execute(commandParts)

	if stdoutFile != nil {
		stdoutFile.Close()
		os.Stdout = originalStdout
	}
	if stderrFile != nil {
		stderrFile.Close()
		os.Stderr = originalStderr
	}
	if stdinFile != nil {
		stdinFile.Close()
		os.Stdin = originalStdin
	}
}

func runExecutable(cmdName string, args []string, redirections []Redirection) {
	executable := exec.Command(cmdName, args...)
	executable.Stdin = os.Stdin
	executable.Stdout = os.Stdout
	executable.Stderr = os.Stderr

	for _, redir := range redirections {
		switch redir.Type {
		case ">", "1>":
			file, err := os.Create(redir.Filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating file %s: %v\n", redir.Filename, err)
				return
			}
			executable.Stdout = file
		case ">>", "1>>":
			file, err := os.OpenFile(redir.Filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening file %s: %v\n", redir.Filename, err)
				return
			}
			executable.Stdout = file
		case "<":
			file, err := os.Open(redir.Filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening file %s: %v\n", redir.Filename, err)
				return
			}
			executable.Stdin = file
		case "2>":
			file, err := os.Create(redir.Filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating file %s: %v\n", redir.Filename, err)
				return
			}
			executable.Stderr = file
		case "2>>":
			file, err := os.OpenFile(redir.Filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening file %s: %v\n", redir.Filename, err)
				return
			}
			executable.Stderr = file
		case "&>", ">&":
			file, err := os.Create(redir.Filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating file %s: %v\n", redir.Filename, err)
				return
			}
			executable.Stdout = file
			executable.Stderr = file
		}
	}

	executable.Run()
}

func getPathDirs() []string {
	pathDirsOnce.Do(func() {
		envPath := os.Getenv("PATH")
		pathDirs = strings.Split(envPath, ":")
	})

	return pathDirs
}

func parseCommand(command string) ParsedCommand {
	state := &parserState{
		parts:        make([]string, 0, 8),
		redirections: make([]Redirection, 0, 2),
		runes:        []rune(command),
		pos:          0,
		inQuote:      false,
	}
	state.current.Grow(16)

	for state.pos < len(state.runes) {
		char := state.runes[state.pos]

		if state.inQuote {
			state.handleQuotedChar(char)
		} else {
			state.handleUnquotedChar(char)
		}
	}

	state.finishCurrentPart()

	return ParsedCommand{
		Parts:        state.parts,
		Redirections: state.redirections,
	}
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

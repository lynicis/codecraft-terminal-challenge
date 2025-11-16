package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"
)

type handleCmd func(commandParts []string)

var builtins map[string]handleCmd

var (
	pathDirs     []string
	pathDirsOnce sync.Once
)

var (
	execCache   = make(map[string]string)
	execCacheMu sync.RWMutex
)

func init() {
	builtins = map[string]handleCmd{
		"exit": handleExitCmd,
		"echo": handleEchoCmd,
		"type": handleTypeCmd,
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
		exitCode = unsafeAtoi(commandParts[1])
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
	execCacheMu.RLock()
	if abs, ok := execCache[cmdName]; ok {
		execCacheMu.RUnlock()
		if stat, err := os.Stat(abs); err == nil && stat.Mode()&0111 != 0 {
			return abs, true
		}

		execCacheMu.Lock()
		delete(execCache, cmdName)
		execCacheMu.Unlock()
	} else {
		execCacheMu.RUnlock()
	}

	directories := getPathDirs()

	for _, dir := range directories {
		abs := filepath.Join(filepath.Clean(dir), cmdName)
		stat, err := os.Stat(abs)
		if err == nil && stat.Mode()&0111 != 0 {
			execCacheMu.Lock()
			execCache[cmdName] = abs
			execCacheMu.Unlock()
			return abs, true
		}
	}

	return "", false
}

func unsafeAtoi(str string) int {
	return *(*int)(unsafe.Pointer(uintptr(unsafe.Pointer(&str))))
}

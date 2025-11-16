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

	"golang.org/x/term"
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
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		reader := bufio.NewReader(os.Stdin)
		for {
			command, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			command = strings.TrimRight(command, "\n\r")
			executeCommand(command)
		}
		return
	}

	for {
		command := readLineWithCompletion()
		if command == "" {
			continue
		}
		executeCommand(command)
	}
}

func executeCommand(command string) {
	parsed := parseCommand(command)
	if len(parsed.Parts) == 0 {
		return
	}

	cmdName := parsed.Parts[0]
	if execute, ok := builtins[cmdName]; ok {
		applyRedirectionsToBuiltin(execute, parsed.Parts, parsed.Redirections)
		return
	}

	_, ok := findProgramInEnv(cmdName)
	if ok {
		runExecutable(cmdName, parsed.Parts[1:], parsed.Redirections)
		return
	}

	fmt.Printf("%s: command not found\n", cmdName)
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

func applyRedirection(redir Redirection, cmd *exec.Cmd) error {
	var file *os.File
	var err error

	switch redir.Type {
	case ">", "1>":
		file, err = os.Create(redir.Filename)
		if err != nil {
			return fmt.Errorf("error creating file %s: %v", redir.Filename, err)
		}
		cmd.Stdout = file
	case ">>", "1>>":
		file, err = os.OpenFile(redir.Filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("error opening file %s: %v", redir.Filename, err)
		}
		cmd.Stdout = file
	case "<":
		file, err = os.Open(redir.Filename)
		if err != nil {
			return fmt.Errorf("error opening file %s: %v", redir.Filename, err)
		}
		cmd.Stdin = file
	case "2>":
		file, err = os.Create(redir.Filename)
		if err != nil {
			return fmt.Errorf("error creating file %s: %v", redir.Filename, err)
		}
		cmd.Stderr = file
	case "2>>":
		file, err = os.OpenFile(redir.Filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("error opening file %s: %v", redir.Filename, err)
		}
		cmd.Stderr = file
	case "&>", ">&":
		file, err = os.Create(redir.Filename)
		if err != nil {
			return fmt.Errorf("error creating file %s: %v", redir.Filename, err)
		}
		cmd.Stdout = file
		cmd.Stderr = file
	}
	return nil
}

func runExecutable(cmdName string, args []string, redirections []Redirection) {
	executable := exec.Command(cmdName, args...)
	executable.Stdin = os.Stdin
	executable.Stdout = os.Stdout
	executable.Stderr = os.Stderr

	for _, redir := range redirections {
		if err := applyRedirection(redir, executable); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return
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

func readLineWithCompletion() string {
	fmt.Fprint(os.Stdout, "$ ")

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		return strings.TrimRight(line, "\n\r")
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	var line []rune
	var cursorPos int
	reader := bufio.NewReader(os.Stdin)

	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			break
		}

		switch r {
		case '\r', '\n':
			fmt.Fprint(os.Stdout, "\r\n")
			return string(line)
		case '\t':
			handleTabCompletion(&line, &cursorPos)
		case 127, 8:
			if cursorPos > 0 {
				line = append(line[:cursorPos-1], line[cursorPos:]...)
				cursorPos--
				redrawLine(string(line), cursorPos)
			}
		case 3:
			fmt.Fprint(os.Stdout, "^C\r\n")
			line = nil
			cursorPos = 0
			fmt.Fprint(os.Stdout, "$ ")
		case 27:
			handleEscapeSequence(reader, &line, &cursorPos)
		default:
			line = append(line, 0)
			copy(line[cursorPos+1:], line[cursorPos:])
			line[cursorPos] = r
			cursorPos++
			redrawLine(string(line), cursorPos)
		}
	}

	return string(line)
}

func handleTabCompletion(line *[]rune, cursorPos *int) {
	lineStr := string(*line)
	completions := getCompletions(lineStr, *cursorPos)
	if len(completions) == 1 {
		completion := completions[0]
		prefixRunes := (*line)[:*cursorPos]
		suffixRunes := (*line)[*cursorPos:]
		wordStartRune := findWordStartRune(prefixRunes)
		wordRunes := prefixRunes[wordStartRune:]

		isCommandCompletion := wordStartRune == 0
		if isCommandCompletion {
			completion = completion + " "
		}

		completionRunes := []rune(completion)
		wordRunesLen := len(wordRunes)

		if len(completionRunes) > wordRunesLen {
			newLine := make([]rune, 0, len(prefixRunes)-wordRunesLen+len(completionRunes)+len(suffixRunes))
			newLine = append(newLine, prefixRunes[:wordStartRune]...)
			newLine = append(newLine, completionRunes...)
			newLine = append(newLine, suffixRunes...)
			*line = newLine
			*cursorPos = wordStartRune + len(completionRunes)
			redrawLine(string(*line), *cursorPos)
		}
	} else if len(completions) > 1 {
		fmt.Fprint(os.Stdout, "\r\n")
		for _, comp := range completions {
			fmt.Fprintf(os.Stdout, "%s  ", comp)
		}
		fmt.Fprint(os.Stdout, "\r\n")
		redrawLine(string(*line), *cursorPos)
	}
}

func handleEscapeSequence(reader *bufio.Reader, line *[]rune, cursorPos *int) {
	next, _ := reader.Peek(2)
	if len(next) >= 2 && next[0] == '[' {
		reader.Discard(2)
		if next[1] == 'C' && *cursorPos < len(*line) {
			*cursorPos++
			redrawLine(string(*line), *cursorPos)
		} else if next[1] == 'D' && *cursorPos > 0 {
			*cursorPos--
			redrawLine(string(*line), *cursorPos)
		}
	}
}

func redrawLine(line string, cursorPos int) {
	fmt.Fprint(os.Stdout, "\r\x1b[K$ "+line)
	runes := []rune(line)
	if cursorPos < len(runes) {
		charsAfterCursor := len(runes) - cursorPos
		if charsAfterCursor > 0 {
			fmt.Fprintf(os.Stdout, "\x1b[%dD", charsAfterCursor)
		}
	}
	os.Stdout.Sync()
}

func findWordStartRune(prefix []rune) int {
	for i := len(prefix) - 1; i >= 0; i-- {
		if prefix[i] == ' ' || prefix[i] == '\t' {
			return i + 1
		}
	}
	return 0
}

func getCompletions(line string, cursorPos int) []string {
	runes := []rune(line)
	if cursorPos > len(runes) {
		cursorPos = len(runes)
	}
	prefix := string(runes[:cursorPos])

	if !strings.Contains(prefix, " ") && !strings.Contains(prefix, "\t") {
		return getCommandCompletions(prefix)
	}

	lastSpaceIdx := strings.LastIndexAny(prefix, " \t")
	if lastSpaceIdx == -1 {
		return getFileCompletions(prefix)
	}
	lastPart := prefix[lastSpaceIdx+1:]
	return getFileCompletions(lastPart)
}

func getCommandCompletions(prefix string) []string {
	var completions []string

	for cmd := range builtins {
		if strings.HasPrefix(cmd, prefix) {
			completions = append(completions, cmd)
		}
	}

	directories := getPathDirs()
	seen := make(map[string]bool)

	for _, dir := range directories {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if strings.HasPrefix(name, prefix) && !seen[name] {
				abs := filepath.Join(dir, name)
				stat, err := os.Stat(abs)
				if err == nil && stat.Mode()&0111 != 0 {
					completions = append(completions, name)
					seen[name] = true
				}
			}
		}
	}

	return completions
}

func getFileCompletions(prefix string) []string {
	var completions []string

	dir := "."
	filePrefix := prefix

	if prefix != "" {
		if strings.HasPrefix(prefix, "~") {
			home := os.Getenv("HOME")
			if home != "" {
				prefix = home + prefix[1:]
			}
		}

		if strings.Contains(prefix, "/") {
			dir = filepath.Dir(prefix)
			filePrefix = filepath.Base(prefix)
		} else {
			dir = "."
			filePrefix = prefix
		}
	}

	if dir == "~" {
		home := os.Getenv("HOME")
		if home != "" {
			dir = home
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return completions
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, filePrefix) {
			if entry.IsDir() {
				completions = append(completions, name+"/")
			} else {
				completions = append(completions, name)
			}
		}
	}

	if dir != "." || strings.Contains(prefix, "/") {
		baseDir := dir
		if baseDir == "." {
			baseDir = filepath.Dir(prefix)
		}
		if baseDir == "" {
			baseDir = "."
		}

		var adjustedCompletions []string
		for _, comp := range completions {
			if strings.HasPrefix(comp, "/") {
				adjustedCompletions = append(adjustedCompletions, comp)
			} else {
				adjustedCompletions = append(adjustedCompletions, filepath.Join(baseDir, comp))
			}
		}
		completions = adjustedCompletions
	}

	return completions
}

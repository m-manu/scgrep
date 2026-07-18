package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const defaultGrepCommand = "grep"
const defaultBatchSize = 20

func getParallelism() int {
	parallelism := runtime.NumCPU() - 1
	if parallelism <= 0 {
		parallelism = 1
	}
	return parallelism
}

const helpMessage = `scgrep - source code grep

Usage:
	scgrep [grep-flags] PATTERN DIRECTORY_PATH

scgrep is a light-weight wrapper for the grep command that only searches
source code files. It traverses the given directory, identifies source code
files by extension and name, skips non-source directories (e.g. .git, node_modules),
and runs grep in parallel across the discovered files.

Arguments:
	PATTERN         The search pattern (passed directly to grep)
	DIRECTORY_PATH  Path to a readable directory to search

All flags except the last two arguments are passed directly to grep.

Examples:
	scgrep "TODO" .
	scgrep -i "fixme" ~/Projects
	scgrep -n --color "func main" /path/to/code

For more details: https://github.com/m-manu/scgrep`

func main() {
	args := os.Args[1:]

	// Handle help flag
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(helpMessage)
		if len(args) == 0 {
			os.Exit(exitCodeInvalidNumArgs)
		}
		os.Exit(exitCodeSuccess)
	}

	// Need at least PATTERN and DIRECTORY_PATH
	if len(args) < 2 {
		showErrorMessageAndExit("error: at least 2 arguments required (PATTERN and DIRECTORY_PATH)", exitCodeInvalidNumArgs)
	}

	var grepArgs []string
	var positionals []string
	skipNext := false

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.Contains(arg, "r") {
			showErrorMessageAndExit("error: all searches are recursive by default; do not pass '-r' flag", exitCodeInvalidNumArgs)
		}

		if skipNext {
			grepArgs = append(grepArgs, arg)
			skipNext = false
			continue
		}

		if strings.HasPrefix(arg, "-") {
			grepArgs = append(grepArgs, arg)
			if arg == "-A" || arg == "-B" || arg == "-C" || arg == "-m" || arg == "-f" || arg == "--context" || arg == "--max-count" || arg == "--file" {
				skipNext = true
			}
		} else {
			positionals = append(positionals, arg)
		}
	}

	if len(positionals) > 2 {
		showErrorMessageAndExit("error: completely invalid format. exactly 1 pattern and 1 directory expected", exitCodeInvalidNumArgs)
	} else if len(positionals) < 2 {
		showErrorMessageAndExit("error: at least 2 non-flag arguments required (PATTERN and DIRECTORY_PATH)", exitCodeInvalidNumArgs)
	}

	dirPath := positionals[1]
	// the pattern goes to grep along with its flags
	grepArgs = append(grepArgs, positionals[0])

	// Resolve symlinks
	resolvedPath, errSL := filepath.EvalSymlinks(dirPath)
	if errSL != nil {
		showErrorMessageAndExit(fmt.Sprintf("error: unable to resolve path \"%s\": %v", dirPath, errSL), exitCodeSymLinkEvalFailed)
	}

	// Validate directory
	if errVD := checkDirectoryIsReadable(resolvedPath); errVD != nil {
		showErrorMessageAndExit(fmt.Sprintf("error: \"%s\" is not a readable directory: %v", dirPath, errVD), exitCodeInputDirectoryNotReadable)
	}

	// Find grep command
	grepCmd, errGrep := findGrepCommand()
	if errGrep != nil {
		showErrorMessageAndExit(fmt.Sprintf("error: %v", errGrep), exitCodeInvalidExecutable)
	}

	// Create channel for file paths
	pathsChan := make(chan string, defaultBatchSize*getParallelism())

	isGitAvailable := false
	if _, errLP := exec.LookPath("git"); errLP == nil {
		isGitAvailable = true
	}

	isGitRepo := false
	if isGitAvailable {
		cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
		cmd.Dir = resolvedPath
		if errC := cmd.Run(); errC == nil {
			isGitRepo = true
		}
	}

	// Start scanner in a goroutine — this is the "main thread" scanner per spec
	scanDone := make(chan error, 1)
	go func() {
		var scanErr error
		if isGitRepo {
			scanErr = scanDirectoryGit(resolvedPath, func(path string) {
				pathsChan <- path
			})
		} else {
			scanErr = scanDirectory(resolvedPath, func(path string) {
				pathsChan <- path
			})
		}
		close(pathsChan)
		scanDone <- scanErr
	}()

	// Run searcher goroutines
	matchFound := runSearchers(getParallelism(), grepCmd, grepArgs, pathsChan, defaultBatchSize)

	// Check for scan errors
	if scanErr := <-scanDone; scanErr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "warning: scan error: %v\n", scanErr)
	}

	// Exit with grep-compatible exit codes
	if matchFound {
		os.Exit(exitCodeSuccess)
	}
	os.Exit(exitCodeNoMatch)
}

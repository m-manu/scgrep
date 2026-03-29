package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

	// Last argument is the directory path, everything else goes to grep
	dirPath := args[len(args)-1]
	grepArgs := args[:len(args)-1] // includes flags + pattern

	// Resolve symlinks
	resolvedPath, err := filepath.EvalSymlinks(dirPath)
	if err != nil {
		showErrorMessageAndExit(fmt.Sprintf("error: unable to resolve path \"%s\": %v", dirPath, err), exitCodeSymLinkEvalFailed)
	}

	// Validate directory
	if err := checkDirectoryIsReadable(resolvedPath); err != nil {
		showErrorMessageAndExit(fmt.Sprintf("error: \"%s\" is not a readable directory: %v", dirPath, err), exitCodeInputDirectoryNotReadable)
	}

	// Find grep command
	grepCmd, err := findGrepCommand()
	if err != nil {
		showErrorMessageAndExit(fmt.Sprintf("error: %v", err), exitCodeInvalidExecutable)
	}

	// Create channel for file paths
	pathsChan := make(chan string, defaultBatchSize*getParallelism())

	// Start scanner in a goroutine — this is the "main thread" scanner per spec
	scanDone := make(chan error, 1)
	go func() {
		scanErr := scanDirectory(resolvedPath, func(path string) {
			pathsChan <- path
		})
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

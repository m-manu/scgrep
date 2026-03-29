package main

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
)

// runSearchers starts 'parallelism' goroutines that consume file paths from the
// channel, batch them, and exec grep on each batch. It returns true if any
// grep invocation found a match (exit code 0).
func runSearchers(parallelism int, grepCmd string, grepArgs []string, paths <-chan string, batchSize int) bool {
	var matchFound atomic.Bool
	var wg sync.WaitGroup

	for range parallelism {
		wg.Go(func() {
			batch := make([]string, 0, batchSize)
			for path := range paths {
				batch = append(batch, path)
				if len(batch) >= batchSize {
					if execGrep(grepCmd, grepArgs, batch) {
						matchFound.Store(true)
					}
					batch = batch[:0]
				}
			}
			// Flush remaining paths
			if len(batch) > 0 {
				if execGrep(grepCmd, grepArgs, batch) {
					matchFound.Store(true)
				}
			}
		})
	}

	wg.Wait()
	return matchFound.Load()
}

// execGrep runs the grep command with the given arguments and file paths.
// Returns true if grep found matches (exit code 0).
func execGrep(grepCmd string, grepArgs []string, filePaths []string) bool {
	// -H forces grep to print filenames even with a single file, which is important
	// since scgrep batches files and a batch may contain only one file.
	args := make([]string, 0, 1+len(grepArgs)+len(filePaths))
	args = append(args, "-H")
	args = append(args, grepArgs...)
	args = append(args, filePaths...)

	cmd := exec.Command(grepCmd, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// grep exit code 1 means no match — that's normal, not an error
			if exitErr.ExitCode() == 1 {
				return false
			}
			// exit code 2 means an actual error
			_, _ = fmt.Fprintf(os.Stderr, "grep error: %v\n", err)
			return false
		}
		_, _ = fmt.Fprintf(os.Stderr, "failed to run grep: %v\n", err)
		return false
	}
	return true
}

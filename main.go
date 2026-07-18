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

Environment:
	GREP_CMD        Override the grep executable (default: grep)
	                e.g. GREP_CMD=fgrep scgrep "LinkedHashSet" .

Examples:
	scgrep "TODO" .
	scgrep -i "fixme" ~/Projects
	scgrep -n --color "func main" /path/to/code

For more details: https://github.com/m-manu/scgrep`

// grepFlagsTakingArg are short/long grep flags whose following argument is a
// value for the flag (not a positional PATTERN/DIRECTORY).
var grepFlagsTakingArg = map[string]bool{
	"-A": true, "--after-context": true,
	"-B": true, "--before-context": true,
	"-C": true, "--context": true,
	"-m": true, "--max-count": true,
	"-e": true, "--regexp": true,
	"-f": true, "--file": true,
	"-d": true, "--directories": true,
	"-D": true, "--devices": true,
	"--label":        true,
	"--binary-files": true,
	"--include":      true,
	"--exclude":      true,
	"--exclude-dir":  true,
	"--exclude-from": true,
}

// hasRecursiveShortFlag reports whether a short-flag cluster (e.g. -rni, -rn)
// enables grep's recursive mode (-r or -R).
func hasRecursiveShortFlag(arg string) bool {
	if !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") {
		return false
	}
	// arg is like "-rni" or "-r"; skip the leading '-'
	for _, c := range arg[1:] {
		if c == 'r' || c == 'R' {
			return true
		}
	}
	return false
}

func isRecursiveLongFlag(arg string) bool {
	return arg == "--recursive" || strings.HasPrefix(arg, "--recursive=")
}

// patternFlags indicate the search pattern is already supplied via a flag
// (-e/--regexp or -f/--file), so only DIRECTORY_PATH is required as a positional.
var patternFlags = map[string]bool{
	"-e": true, "--regexp": true,
	"-f": true, "--file": true,
}

// parseArgs splits CLI args into grep flags and positionals (pattern + directory,
// or directory only when the pattern was given via -e/-f). Flags that take a
// value consume the next argument. A bare "--" ends flag parsing.
func parseArgs(args []string) (grepArgs, positionals []string, errMsg string) {
	skipNext := false
	endOfFlags := false
	patternFromFlag := false

	for _, arg := range args {
		if endOfFlags {
			positionals = append(positionals, arg)
			continue
		}
		if skipNext {
			grepArgs = append(grepArgs, arg)
			skipNext = false
			continue
		}
		if arg == "--" {
			endOfFlags = true
			grepArgs = append(grepArgs, arg)
			continue
		}

		if hasRecursiveShortFlag(arg) || isRecursiveLongFlag(arg) {
			return nil, nil, "error: all searches are recursive by default; do not pass '-r' flag"
		}

		if strings.HasPrefix(arg, "-") {
			name, _, hasEq := strings.Cut(arg, "=")
			grepArgs = append(grepArgs, arg)
			if patternFlags[name] {
				patternFromFlag = true
			}
			if !hasEq && grepFlagsTakingArg[name] {
				skipNext = true
			}
			// Glued short forms like -C2 / -A10: value is in the same token
			if !hasEq && len(arg) > 2 && (arg[1] == 'A' || arg[1] == 'B' || arg[1] == 'C' || arg[1] == 'm') {
				rest := arg[2:]
				if rest != "" && rest[0] >= '0' && rest[0] <= '9' {
					skipNext = false
				}
			}
		} else {
			positionals = append(positionals, arg)
		}
	}

	if skipNext {
		return nil, nil, "error: flag requires an argument"
	}

	if patternFromFlag {
		// Pattern already provided via -e/-f: only DIRECTORY_PATH is required
		if len(positionals) == 0 {
			return nil, nil, "error: directory path required (PATTERN was given via -e/-f)"
		}
		if len(positionals) > 1 {
			return nil, nil, "error: completely invalid format. exactly 1 directory expected when using -e/-f"
		}
		return grepArgs, positionals, ""
	}

	if len(positionals) > 2 {
		return nil, nil, "error: completely invalid format. exactly 1 pattern and 1 directory expected"
	}
	if len(positionals) < 2 {
		return nil, nil, "error: at least 2 non-flag arguments required (PATTERN and DIRECTORY_PATH)"
	}
	return grepArgs, positionals, ""
}

func isGitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func isInsideGitWorkTree(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	return cmd.Run() == nil
}

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

	grepArgs, positionals, errMsg := parseArgs(args)
	if errMsg != "" {
		showErrorMessageAndExit(errMsg, exitCodeInvalidNumArgs)
	}

	// positionals: [pattern, dir] normally, or [dir] when pattern came from -e/-f
	var dirPath string
	if len(positionals) == 2 {
		dirPath = positionals[1]
		grepArgs = append(grepArgs, positionals[0])
	} else {
		dirPath = positionals[0]
	}

	// Resolve symlinks
	resolvedPath, errSL := filepath.EvalSymlinks(dirPath)
	if errSL != nil {
		showErrorMessageAndExit(fmt.Sprintf("error: unable to resolve path \"%s\": %v", dirPath, errSL), exitCodeSymLinkEvalFailed)
	}

	// Validate directory
	if errVD := checkDirectoryIsReadable(resolvedPath); errVD != nil {
		showErrorMessageAndExit(fmt.Sprintf("error: \"%s\" is not a readable directory: %v", dirPath, errVD), exitCodeInputDirectoryNotReadable)
	}

	// Find grep command (honours GREP_CMD)
	grepCmd, errGrep := findGrepCommand()
	if errGrep != nil {
		showErrorMessageAndExit(fmt.Sprintf("error: %v", errGrep), exitCodeInvalidExecutable)
	}

	// Check git availability early
	gitOK := isGitAvailable()
	useGit := gitOK && isInsideGitWorkTree(resolvedPath)

	// Create channel for file paths
	pathsChan := make(chan string, defaultBatchSize*getParallelism())

	// Start scanner in a goroutine
	scanDone := make(chan error, 1)
	go func() {
		var scanErr error
		if useGit {
			scanErr = scanDirectoryGit(resolvedPath, func(path string) {
				pathsChan <- path
			})
			if scanErr != nil {
				// Fallback to filesystem walk if git ls-files fails
				_, _ = fmt.Fprintf(os.Stderr, "warning: git scan failed, falling back to directory walk: %v\n", scanErr)
				scanErr = scanDirectory(resolvedPath, func(path string) {
					pathsChan <- path
				}, gitOK)
			}
		} else {
			scanErr = scanDirectory(resolvedPath, func(path string) {
				pathsChan <- path
			}, gitOK)
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

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getFileExt(fileName string) string {
	fileExt := strings.ReplaceAll(
		strings.ToLower(
			filepath.Ext(fileName),
		),
		".", "")
	return fileExt
}

func checkDirectoryIsReadable(path string) error {
	fileInfo, statErr := os.Stat(path)
	if statErr != nil {
		return statErr
	}
	if !fileInfo.IsDir() {
		return errors.New("not a directory")
	}
	return nil
}

func doesFileExist(path string) bool {
	info, err := os.Stat(path)
	return err == nil &&
		info.Mode().IsRegular()
}

func showErrorMessageAndExit(msg string, exitCode int) {
	_, _ = fmt.Fprintf(os.Stderr, "%s\n%s\n", msg, "Run `scgrep -h` for usage")
	os.Exit(exitCode)
}

// findGrepCommand locates the grep executable on the system.
func findGrepCommand() (string, error) {
	path, err := exec.LookPath(defaultGrepCommand)
	if err != nil {
		return "", fmt.Errorf("grep command not found: %w", err)
	}
	return path, nil
}

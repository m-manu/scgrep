package main

import (
	"testing"
)

func TestConfigFiles(t *testing.T) {
	if allowedFileExtensions.isEmpty() ||
		allowedFileNames.isEmpty() ||
		ignoredDirectories.isEmpty() ||
		len(ignoredDirectoriesWithPeerFileNames) == 0 {
		t.Errorf("one or more config files empty")
	}
}

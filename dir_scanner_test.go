package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestWalk(t *testing.T) {
	var paths []string
	wdErr := scanDirectory(runtime.GOROOT(), func(entry string) {
		paths = append(paths, entry)
	})
	if wdErr != nil {
		t.Error("Walk failed on go root")
	}
	var htmlFilesCount, goFilesCount int
	for _, path := range paths {
		if strings.HasSuffix(path, ".html") {
			htmlFilesCount++
		}
		if strings.HasSuffix(path, ".go") {
			goFilesCount++
		}
	}
	if htmlFilesCount < 2 || goFilesCount < 6500 {
		t.Logf("html: %v, go: %v", htmlFilesCount, goFilesCount)
		t.Error("Directory scanner wasn't able to detect all the source code files")
	}
}

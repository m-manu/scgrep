package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed config/allowed_file_extensions.txt
var allowedFileExtensionsRaw string

//go:embed config/allowed_file_names.txt
var allowedFileNamesRaw string

//go:embed config/ignored_directories.txt
var ignoredDirectoriesRaw string

//go:embed config/ignored_directories_with_peer_file_names.json
var ignoredDirectoriesWithPeerFileNamesRaw []byte

var allowedFileExtensions set[string]
var allowedFileNames set[string]
var ignoredDirectories set[string]
var ignoredDirectoriesWithPeerFileNames map[string][]string

/**
Note: Calls to panic function in this file are meant for catching 'bugs related to configuration files' during `go test`
*/

// Convert a line-delimited string to a lookup map
func toLookupMap(contents string, fileName string) set[string] {
	entries := newSet[string](100)
	lines := strings.Split(contents, "\n")
	for lineNumber, lineText := range lines {
		if strings.TrimSpace(lineText) == "" {
			panic(fmt.Sprintf("Issue in file %s: Line %d is empty", fileName, lineNumber+1))
		}
		if entries.contains(lineText) {
			panic(fmt.Sprintf("Issue in file %s at line %d: Entry \"%s\" is repeated",
				fileName, lineNumber+1, lineText),
			)
		}
		entries.add(lineText)
	}
	return entries
}

// Initialize the lookup maps
func init() {
	allowedFileExtensions = toLookupMap(allowedFileExtensionsRaw, "allowed_file_extensions.txt")
	allowedFileNames = toLookupMap(allowedFileNamesRaw, "allowed_file_names.txt")
	ignoredDirectories = toLookupMap(ignoredDirectoriesRaw, "ignored_directories.txt")
	err := json.Unmarshal(ignoredDirectoriesWithPeerFileNamesRaw, &ignoredDirectoriesWithPeerFileNames)
	if err != nil {
		panic(fmt.Sprintf("Unable to parse config file ignored_directories_with_peer_file_names.json: %+v", err))
	}
}

// Constants for exit codes
const (
	exitCodeSuccess        = iota // 0: grep found a match
	exitCodeNoMatch               // 1: grep found no matches (matches grep convention)
	exitCodeInvalidNumArgs        // 2: invalid number of arguments
	exitCodeInputDirectoryNotReadable
	exitCodeSymLinkEvalFailed
	exitCodeInvalidExecutable
)

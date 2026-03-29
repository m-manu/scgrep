package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func scanDirectory(dir string, emit func(string)) error {
	walkFn := func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "skipping \"%s\": %+v\n", path, err)
			return nil
		}
		// If the directory is in excluded directories list, skip it
		if de.IsDir() && ignoredDirectories.contains(de.Name()) {
			return filepath.SkipDir
		}
		// If the directory has a peer file that triggers ignoring, skip it
		if de.IsDir() {
			if peerFiles, exists := ignoredDirectoriesWithPeerFileNames[de.Name()]; exists {
				for _, peerFile := range peerFiles {
					peerFilePath := filepath.Join(filepath.Dir(path), peerFile)
					if doesFileExist(peerFilePath) {
						return filepath.SkipDir
					}
				}
			}
			return nil
		}
		// Ignore dot files (Mac) and non-regular files
		if strings.HasPrefix(de.Name(), "._") || !de.Type().IsRegular() {
			return nil
		}
		// Emit source code files
		if allowedFileExtensions.contains(getFileExt(de.Name())) || allowedFileNames.contains(de.Name()) {
			emit(path)
		}
		return nil
	}
	return filepath.WalkDir(dir, walkFn)
}

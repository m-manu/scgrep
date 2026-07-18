package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// isGitRepoRoot reports whether path is the root of a Git working tree
// (has a .git file or directory). Faster than invoking git for every directory.
func isGitRepoRoot(path string) bool {
	_, err := os.Lstat(filepath.Join(path, ".git"))
	return err == nil
}

// pathHasIgnoredDir reports whether any path component is an always-ignored
// directory (e.g. .git, .idea).
func pathHasIgnoredDir(relPath string) bool {
	for _, part := range strings.Split(relPath, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		if ignoredDirectories.contains(part) {
			return true
		}
	}
	return false
}

func shouldEmitSourceFile(baseName string) bool {
	if strings.HasPrefix(baseName, "._") {
		return false
	}
	return allowedFileExtensions.contains(getFileExt(baseName)) || allowedFileNames.contains(baseName)
}

// scanDirectory walks dir for source-code files. When gitAvailable is true and
// a nested directory is a Git repo root, that subtree is scanned via git
// (respecting its .gitignore) instead of a plain walk.
func scanDirectory(dir string, emit func(string), gitAvailable bool) error {
	walkFn := func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "skipping \"%s\": %+v\n", path, err)
			return nil
		}

		if de.IsDir() {
			// Nested Git repository: prefer git ls-files so .gitignore is honoured
			// (skip the search root itself — caller already chose walk mode for it).
			if gitAvailable && path != dir && isGitRepoRoot(path) {
				if gitErr := scanDirectoryGit(path, emit); gitErr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "warning: git scan failed for \"%s\", walking instead: %v\n", path, gitErr)
					return nil // continue walking this tree
				}
				return filepath.SkipDir
			}

			// If the directory is in excluded directories list, skip it
			if ignoredDirectories.contains(de.Name()) {
				return filepath.SkipDir
			}
			// If the directory has a peer file that triggers ignoring, skip it
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

		// Ignore Mac resource-fork files and non-regular files
		if strings.HasPrefix(de.Name(), "._") || !de.Type().IsRegular() {
			return nil
		}
		// Emit source code files
		if shouldEmitSourceFile(de.Name()) {
			emit(path)
		}
		return nil
	}
	return filepath.WalkDir(dir, walkFn)
}

// scanDirectoryGit lists source files via `git ls-files`, which respects
// .gitignore. Paths under always-ignored directories are still filtered out.
func scanDirectoryGit(dir string, emit func(string)) error {
	cmd := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return err
	}

	files := strings.Split(string(out), "\x00")
	for _, file := range files {
		if file == "" {
			continue
		}

		// Skip paths under always-ignored directories (.git, .idea, …)
		if pathHasIgnoredDir(file) {
			continue
		}

		baseName := filepath.Base(file)
		if !shouldEmitSourceFile(baseName) {
			continue
		}

		fullPath := filepath.Join(dir, file)
		// Skip missing paths, broken symlinks, and non-regular files so grep
		// is not invoked on paths it cannot open.
		info, errStat := os.Lstat(fullPath)
		if errStat != nil || !info.Mode().IsRegular() {
			continue
		}
		emit(fullPath)
	}
	return nil
}

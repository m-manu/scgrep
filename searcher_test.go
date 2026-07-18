package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecGrep(t *testing.T) {
	// Create a temp directory with a known file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "hello.go")
	if err := os.WriteFile(testFile, []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	grepCmd, err := exec.LookPath("grep")
	if err != nil {
		t.Skip("grep not found, skipping test")
	}

	t.Run("match found", func(t *testing.T) {
		found := execGrep(grepCmd, []string{"package"}, []string{testFile})
		if !found {
			t.Error("expected match to be found")
		}
	})

	t.Run("no match", func(t *testing.T) {
		found := execGrep(grepCmd, []string{"nonexistentpattern12345"}, []string{testFile})
		if found {
			t.Error("expected no match")
		}
	})
}

func TestRunSearchers(t *testing.T) {
	// Create a temp directory with several source code files
	tmpDir := t.TempDir()
	for i := 0; i < 5; i++ {
		filename := filepath.Join(tmpDir, "file"+string(rune('a'+i))+".go")
		content := "package main\n// MARKER_" + string(rune('A'+i)) + "\nfunc init() {}\n"
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	grepCmd, err := exec.LookPath("grep")
	if err != nil {
		t.Skip("grep not found, skipping test")
	}

	t.Run("matches across files", func(t *testing.T) {
		paths := make(chan string, 10)
		for i := 0; i < 5; i++ {
			paths <- filepath.Join(tmpDir, "file"+string(rune('a'+i))+".go")
		}
		close(paths)

		found := runSearchers(2, grepCmd, []string{"MARKER"}, paths, 3)
		if !found {
			t.Error("expected matches to be found")
		}
	})

	t.Run("no matches", func(t *testing.T) {
		paths := make(chan string, 10)
		for i := 0; i < 5; i++ {
			paths <- filepath.Join(tmpDir, "file"+string(rune('a'+i))+".go")
		}
		close(paths)

		found := runSearchers(2, grepCmd, []string{"ZZZZNOTFOUND"}, paths, 3)
		if found {
			t.Error("expected no matches")
		}
	})

	t.Run("empty channel", func(t *testing.T) {
		paths := make(chan string)
		close(paths)

		found := runSearchers(2, grepCmd, []string{"anything"}, paths, 3)
		if found {
			t.Error("expected no matches on empty channel")
		}
	})
}

func TestFindGrepCommand(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("GREP_CMD", "")
		path, err := findGrepCommand()
		if err != nil {
			t.Skipf("grep not available: %v", err)
		}
		if path == "" {
			t.Error("expected non-empty path for grep")
		}
		if !strings.Contains(path, "grep") {
			t.Errorf("expected path to contain 'grep', got: %s", path)
		}
	})

	t.Run("GREP_CMD override missing", func(t *testing.T) {
		t.Setenv("GREP_CMD", "definitely_not_a_grep_cmd_xyz")
		_, err := findGrepCommand()
		if err == nil {
			t.Fatal("expected error for missing GREP_CMD")
		}
		if !strings.Contains(err.Error(), "definitely_not_a_grep_cmd_xyz") {
			t.Errorf("error should name the command: %v", err)
		}
	})

	t.Run("GREP_CMD override valid", func(t *testing.T) {
		grepPath, err := exec.LookPath("grep")
		if err != nil {
			t.Skip("grep not found")
		}
		t.Setenv("GREP_CMD", grepPath)
		path, errFG := findGrepCommand()
		if errFG != nil {
			t.Fatal(errFG)
		}
		if path != grepPath {
			t.Errorf("got %s, want %s", path, grepPath)
		}
	})
}

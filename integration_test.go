package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegration_EndToEnd builds the binary and runs it against a temp directory
// with known file content, asserting that grep results are correct.
func TestIntegration_EndToEnd(t *testing.T) {
	if _, err := exec.LookPath("grep"); err != nil {
		t.Skip("grep not found, skipping integration test")
	}

	// Build the scgrep binary
	tmpBin := filepath.Join(t.TempDir(), "scgrep")
	buildCmd := exec.Command("go", "build", "-o", tmpBin, ".")
	buildCmd.Dir = "."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build scgrep: %v\n%s", err, out)
	}

	// Create a test directory tree
	testDir := t.TempDir()

	// Source code files (should be searched)
	srcDir := filepath.Join(testDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(srcDir, "main.go"), "package main\n// UNIQUE_MARKER_XYZ\nfunc main() {}\n")
	mustWriteFile(t, filepath.Join(srcDir, "utils.py"), "# UNIQUE_MARKER_XYZ\ndef helper(): pass\n")

	// Non-source file (should NOT be searched)
	mustWriteFile(t, filepath.Join(srcDir, "data.bin"), "UNIQUE_MARKER_XYZ binary stuff")

	// Ignored directory (.git should be skipped)
	gitDir := filepath.Join(testDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(gitDir, "config.go"), "// UNIQUE_MARKER_XYZ\n")

	// Nested directory with source code
	nestedDir := filepath.Join(srcDir, "nested")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(nestedDir, "deep.java"), "// UNIQUE_MARKER_XYZ\nclass Deep {}\n")

	t.Run("finds matches in source code files", func(t *testing.T) {
		cmd := exec.Command(tmpBin, "UNIQUE_MARKER_XYZ", testDir)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			t.Logf("stderr: %s", stderr.String())
			t.Fatalf("expected exit code 0 (match found), got error: %v", err)
		}

		output := stdout.String()

		// Should find matches in .go, .py, .java files
		if !strings.Contains(output, "main.go") {
			t.Error("expected output to contain main.go")
		}
		if !strings.Contains(output, "utils.py") {
			t.Error("expected output to contain utils.py")
		}
		if !strings.Contains(output, "deep.java") {
			t.Error("expected output to contain deep.java")
		}

		// Should NOT contain .bin file or .git directory file
		if strings.Contains(output, "data.bin") {
			t.Error("output should NOT contain data.bin (non-source file)")
		}
		if strings.Contains(output, ".git") {
			t.Error("output should NOT contain .git (ignored directory)")
		}
	})

	t.Run("no match returns exit code 1", func(t *testing.T) {
		cmd := exec.Command(tmpBin, "ZZZZ_NONEXISTENT_PATTERN", testDir)
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected exit code 1 (no match), got 0")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 1 {
				t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
			}
		}
	})

	t.Run("grep flags are passed through", func(t *testing.T) {
		cmd := exec.Command(tmpBin, "-c", "UNIQUE_MARKER_XYZ", testDir)
		var stdout bytes.Buffer
		cmd.Stdout = &stdout

		err := cmd.Run()
		if err != nil {
			t.Fatalf("expected match with -c flag, got: %v", err)
		}

		// -c option makes grep output counts, not the matching lines
		output := stdout.String()
		if strings.Contains(output, "UNIQUE_MARKER_XYZ") {
			t.Error("with -c flag, output should contain counts not matching text")
		}
	})

	t.Run("invalid directory returns error", func(t *testing.T) {
		cmd := exec.Command(tmpBin, "pattern", "/nonexistent/path/xyz")
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected error for invalid directory")
		}
	})

	t.Run("help flag", func(t *testing.T) {
		cmd := exec.Command(tmpBin, "-h")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			t.Fatalf("help flag should exit 0: %v", err)
		}

		output := stdout.String()
		if !strings.Contains(output, "scgrep") {
			t.Error("help output should contain 'scgrep'")
		}
		if !strings.Contains(output, "PATTERN") {
			t.Error("help output should contain 'PATTERN'")
		}
	})

	t.Run("flags not counted as positionals", func(t *testing.T) {
		cmd := exec.Command(tmpBin, "--color", "LinkedHashSet")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected error when directory is missing")
		}
		msg := stderr.String()
		if !strings.Contains(msg, "non-flag arguments") && !strings.Contains(msg, "PATTERN and DIRECTORY") {
			t.Errorf("expected clear missing-args error, got: %s", msg)
		}
		// Must not treat the pattern as a directory path
		if strings.Contains(msg, "unable to resolve path") {
			t.Errorf("should not resolve pattern as path, got: %s", msg)
		}
	})

	t.Run("rejects -r flag", func(t *testing.T) {
		for _, flag := range []string{"-r", "-rni", "-rn"} {
			cmd := exec.Command(tmpBin, flag, "pattern", testDir)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()
			if err == nil {
				t.Errorf("expected error for flag %s", flag)
				continue
			}
			if !strings.Contains(stderr.String(), "recursive by default") {
				t.Errorf("flag %s: expected recursive error, got: %s", flag, stderr.String())
			}
		}
	})

	t.Run("GREP_CMD override", func(t *testing.T) {
		// Valid override: use system grep via env
		grepPath, err := exec.LookPath("grep")
		if err != nil {
			t.Skip("grep not found")
		}
		cmd := exec.Command(tmpBin, "UNIQUE_MARKER_XYZ", testDir)
		cmd.Env = append(os.Environ(), "GREP_CMD="+grepPath)
		if err := cmd.Run(); err != nil {
			t.Fatalf("GREP_CMD=grep path should work: %v", err)
		}

		// Invalid override
		cmd = exec.Command(tmpBin, "UNIQUE_MARKER_XYZ", testDir)
		cmd.Env = append(os.Environ(), "GREP_CMD=not_a_real_grep_binary_xyz")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err = cmd.Run()
		if err == nil {
			t.Fatal("expected error for invalid GREP_CMD")
		}
		if !strings.Contains(stderr.String(), "not_a_real_grep_binary_xyz") {
			t.Errorf("expected GREP_CMD name in error, got: %s", stderr.String())
		}
	})

	t.Run("rejects multiple directories", func(t *testing.T) {
		cmd := exec.Command(tmpBin, "pattern", testDir, testDir)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err == nil {
			t.Fatal("expected error for multiple directories")
		}
		if !strings.Contains(stderr.String(), "exactly 1 pattern and 1 directory") {
			t.Errorf("unexpected error: %s", stderr.String())
		}
	})

	t.Run("rejects file as directory", func(t *testing.T) {
		filePath := filepath.Join(srcDir, "main.go")
		cmd := exec.Command(tmpBin, "pattern", filePath)
		if err := cmd.Run(); err == nil {
			t.Fatal("expected error when path is a file")
		}
	})
}

func TestIntegration_GitIgnore(t *testing.T) {
	if _, err := exec.LookPath("grep"); err != nil {
		t.Skip("grep not found")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	tmpBin := filepath.Join(t.TempDir(), "scgrep")
	buildCmd := exec.Command("go", "build", "-o", tmpBin, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	repo := t.TempDir()
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "test")

	mustWriteFile(t, filepath.Join(repo, ".gitignore"), "ignored.go\n")
	mustWriteFile(t, filepath.Join(repo, "tracked.go"), "package t\n// GIT_MARKER_AAA\n")
	mustWriteFile(t, filepath.Join(repo, "ignored.go"), "package i\n// GIT_MARKER_BBB\n")
	runGit("add", ".gitignore", "tracked.go")
	runGit("commit", "-m", "init")
	// untracked, not ignored
	mustWriteFile(t, filepath.Join(repo, "untracked.go"), "package u\n// GIT_MARKER_CCC\n")

	cmd := exec.Command(tmpBin, "GIT_MARKER", repo)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("expected matches: %v\n%s", err, stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "GIT_MARKER_AAA") {
		t.Error("expected tracked file match")
	}
	if !strings.Contains(out, "GIT_MARKER_CCC") {
		t.Error("expected untracked non-ignored match")
	}
	if strings.Contains(out, "GIT_MARKER_BBB") {
		t.Error("should not match gitignored file")
	}
}

func TestIntegration_NestedGitFromParent(t *testing.T) {
	if _, err := exec.LookPath("grep"); err != nil {
		t.Skip("grep not found")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	tmpBin := filepath.Join(t.TempDir(), "scgrep")
	buildCmd := exec.Command("go", "build", "-o", tmpBin, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	parent := t.TempDir()
	mustWriteFile(t, filepath.Join(parent, "root.go"), "// NEST_MARKER_ROOT\n")

	repo := filepath.Join(parent, "nested")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "test")
	mustWriteFile(t, filepath.Join(repo, ".gitignore"), "hidden.go\n")
	mustWriteFile(t, filepath.Join(repo, "visible.go"), "// NEST_MARKER_VIS\n")
	mustWriteFile(t, filepath.Join(repo, "hidden.go"), "// NEST_MARKER_HID\n")
	runGit("add", ".gitignore", "visible.go")
	runGit("commit", "-m", "init")

	cmd := exec.Command(tmpBin, "NEST_MARKER", parent)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("expected matches: %v\n%s", err, stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "NEST_MARKER_ROOT") {
		t.Error("expected root match")
	}
	if !strings.Contains(out, "NEST_MARKER_VIS") {
		t.Error("expected nested visible match")
	}
	if strings.Contains(out, "NEST_MARKER_HID") {
		t.Error("should not match gitignored file in nested repo when searching from parent")
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

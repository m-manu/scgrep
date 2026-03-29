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
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main\n// UNIQUE_MARKER_XYZ\nfunc main() {}\n"), 0644)
	os.WriteFile(filepath.Join(srcDir, "utils.py"), []byte("# UNIQUE_MARKER_XYZ\ndef helper(): pass\n"), 0644)

	// Non-source file (should NOT be searched)
	os.WriteFile(filepath.Join(srcDir, "data.bin"), []byte("UNIQUE_MARKER_XYZ binary stuff"), 0644)

	// Ignored directory (.git should be skipped)
	gitDir := filepath.Join(testDir, ".git")
	os.MkdirAll(gitDir, 0755)
	os.WriteFile(filepath.Join(gitDir, "config.go"), []byte("// UNIQUE_MARKER_XYZ\n"), 0644)

	// Nested directory with source code
	nestedDir := filepath.Join(srcDir, "nested")
	os.MkdirAll(nestedDir, 0755)
	os.WriteFile(filepath.Join(nestedDir, "deep.java"), []byte("// UNIQUE_MARKER_XYZ\nclass Deep {}\n"), 0644)

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
		cmd.Run() // ignore exit code

		output := stdout.String()
		if !strings.Contains(output, "scgrep") {
			t.Error("help output should contain 'scgrep'")
		}
		if !strings.Contains(output, "PATTERN") {
			t.Error("help output should contain 'PATTERN'")
		}
	})
}

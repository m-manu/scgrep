package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func goRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Fatalf("go env GOROOT: %v", err)
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		t.Fatal("empty GOROOT from go env")
	}
	return root
}

func TestWalk(t *testing.T) {
	var paths []string
	wdErr := scanDirectory(goRoot(t), func(entry string) {
		paths = append(paths, entry)
	}, false)
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

func TestScanDirectory_PeerFileIgnoreBuildGradleKts(t *testing.T) {
	tmp := t.TempDir()
	app := filepath.Join(tmp, "app")
	buildDir := filepath.Join(app, "build")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "build.gradle.kts"), []byte("// gradle"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "Generated.java"), []byte("class Generated {} // MARKER_BUILD"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Main.java"), []byte("class Main {} // MARKER_OK"), 0644); err != nil {
		t.Fatal(err)
	}

	var paths []string
	if err := scanDirectory(tmp, func(p string) { paths = append(paths, p) }, false); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(paths, "\n")
	if strings.Contains(joined, "Generated.java") {
		t.Errorf("build/ should be skipped when build.gradle.kts exists; got: %v", paths)
	}
	if !strings.Contains(joined, "Main.java") {
		t.Errorf("expected Main.java to be scanned; got: %v", paths)
	}
}

func TestScanDirectory_NestedGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	parent := t.TempDir()
	// Non-git parent content
	if err := os.WriteFile(filepath.Join(parent, "top.go"), []byte("package top // MARKER_TOP"), 0644); err != nil {
		t.Fatal(err)
	}

	// Nested git repo with a gitignored source file
	repo := filepath.Join(parent, "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	run("git", "init")
	run("git", "config", "user.email", "test@example.com")
	run("git", "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("ignored.go\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "tracked.go"), []byte("package tracked // MARKER_TRACKED"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "ignored.go"), []byte("package ignored // MARKER_IGNORED"), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".gitignore", "tracked.go")
	run("git", "commit", "-m", "init")

	var paths []string
	if err := scanDirectory(parent, func(p string) { paths = append(paths, p) }, true); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(paths, "\n")
	if !strings.Contains(joined, "top.go") {
		t.Errorf("expected top.go; got %v", paths)
	}
	if !strings.Contains(joined, "tracked.go") {
		t.Errorf("expected tracked.go; got %v", paths)
	}
	if strings.Contains(joined, "ignored.go") {
		t.Errorf("nested gitignore should exclude ignored.go; got %v", paths)
	}
}

func TestScanDirectoryGit_RespectsGitignore(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	run("git", "init")
	run("git", "config", "user.email", "test@example.com")
	run("git", "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("secret.go\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "ok.go"), []byte("package ok"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "secret.go"), []byte("package secret"), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".gitignore", "ok.go")
	run("git", "commit", "-m", "init")

	var paths []string
	if err := scanDirectoryGit(repo, func(p string) { paths = append(paths, p) }); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(paths, "\n")
	if !strings.Contains(joined, "ok.go") {
		t.Errorf("expected ok.go; got %v", paths)
	}
	if strings.Contains(joined, "secret.go") {
		t.Errorf("gitignore should exclude secret.go; got %v", paths)
	}
}

func TestPathHasIgnoredDir(t *testing.T) {
	if !pathHasIgnoredDir(".idea/workspace.xml") {
		t.Error("expected .idea to be ignored")
	}
	if pathHasIgnoredDir("src/main.go") {
		t.Error("src/main.go should not be ignored")
	}
}

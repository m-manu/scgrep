package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantGrep    []string
		wantPos     []string
		wantErrPart string
	}{
		{
			name:     "basic pattern and dir",
			args:     []string{"TODO", "."},
			wantGrep: nil,
			wantPos:  []string{"TODO", "."},
		},
		{
			name:     "flags before pattern",
			args:     []string{"-i", "-n", "--color", "TODO", "/tmp"},
			wantGrep: []string{"-i", "-n", "--color"},
			wantPos:  []string{"TODO", "/tmp"},
		},
		{
			name:     "color equals form",
			args:     []string{"--color=never", "pat", "dir"},
			wantGrep: []string{"--color=never"},
			wantPos:  []string{"pat", "dir"},
		},
		{
			name:     "-A consumes next arg",
			args:     []string{"-A", "2", "pat", "dir"},
			wantGrep: []string{"-A", "2"},
			wantPos:  []string{"pat", "dir"},
		},
		{
			name:     "-e supplies pattern; only directory positional",
			args:     []string{"-e", "hello", "dir"},
			wantGrep: []string{"-e", "hello"},
			wantPos:  []string{"dir"},
		},
		{
			name:     "-e then another flag then directory",
			args:     []string{"-e", "Hello", "-i", "dir"},
			wantGrep: []string{"-e", "Hello", "-i"},
			wantPos:  []string{"dir"},
		},
		{
			name:     "-i -e pattern dir",
			args:     []string{"-i", "-e", "Hello", "dir"},
			wantGrep: []string{"-i", "-e", "Hello"},
			wantPos:  []string{"dir"},
		},
		{
			name:     "-- ends flags so dash patterns work",
			args:     []string{"--", "-weird", "dir"},
			wantGrep: []string{"--"},
			wantPos:  []string{"-weird", "dir"},
		},
		{
			name:        "-e with extra positional is invalid",
			args:        []string{"-e", "hello", "extra", "dir"},
			wantErrPart: "exactly 1 directory expected when using -e/-f",
		},
		{
			name:        "missing directory with flags",
			args:        []string{"--color", "LinkedHashSet"},
			wantErrPart: "at least 2 non-flag arguments",
		},
		{
			name:        "too many positionals",
			args:        []string{"pat", "dir1", "dir2"},
			wantErrPart: "exactly 1 pattern and 1 directory",
		},
		{
			name:        "rejects -r",
			args:        []string{"-r", "pat", "dir"},
			wantErrPart: "recursive by default",
		},
		{
			name:        "rejects -rni composite",
			args:        []string{"-rni", "pat", "dir"},
			wantErrPart: "recursive by default",
		},
		{
			name:        "rejects -rn composite",
			args:        []string{"-rn", "pat", "dir"},
			wantErrPart: "recursive by default",
		},
		{
			name:        "rejects -ir composite",
			args:        []string{"-ir", "pat", "dir"},
			wantErrPart: "recursive by default",
		},
		{
			name:        "rejects -R",
			args:        []string{"-R", "pat", "dir"},
			wantErrPart: "recursive by default",
		},
		{
			name:        "rejects --recursive",
			args:        []string{"--recursive", "pat", "dir"},
			wantErrPart: "recursive by default",
		},
		{
			name:     "allows -n without r",
			args:     []string{"-ni", "pat", "dir"},
			wantGrep: []string{"-ni"},
			wantPos:  []string{"pat", "dir"},
		},
		{
			name:     "glued -C2 does not swallow pattern",
			args:     []string{"-C2", "pat", "dir"},
			wantGrep: []string{"-C2"},
			wantPos:  []string{"pat", "dir"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grepArgs, positionals, errMsg := parseArgs(tt.args)
			if tt.wantErrPart != "" {
				if errMsg == "" {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrPart)
				}
				if !strings.Contains(errMsg, tt.wantErrPart) {
					t.Fatalf("error %q does not contain %q", errMsg, tt.wantErrPart)
				}
				return
			}
			if errMsg != "" {
				t.Fatalf("unexpected error: %s", errMsg)
			}
			if !reflect.DeepEqual(grepArgs, tt.wantGrep) {
				t.Errorf("grepArgs = %#v, want %#v", grepArgs, tt.wantGrep)
			}
			if !reflect.DeepEqual(positionals, tt.wantPos) {
				t.Errorf("positionals = %#v, want %#v", positionals, tt.wantPos)
			}
		})
	}
}

func TestHasRecursiveShortFlag(t *testing.T) {
	if !hasRecursiveShortFlag("-r") {
		t.Error("-r")
	}
	if !hasRecursiveShortFlag("-rni") {
		t.Error("-rni")
	}
	if !hasRecursiveShortFlag("-R") {
		t.Error("-R")
	}
	if hasRecursiveShortFlag("-ni") {
		t.Error("-ni should not be recursive")
	}
	if hasRecursiveShortFlag("--recursive") {
		t.Error("long flags handled separately")
	}
	if hasRecursiveShortFlag("--color") {
		t.Error("--color")
	}
}

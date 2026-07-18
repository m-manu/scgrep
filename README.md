# Source Code GREP (SCGREP)

[![Build and test](https://github.com/m-manu/scgrep/actions/workflows/build-and-test.yml/badge.svg)](https://github.com/m-manu/scgrep/actions/workflows/build-and-test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/m-manu/scgrep.svg)](https://pkg.go.dev/github.com/m-manu/scgrep)
[![License](https://img.shields.io/badge/License-Apache%202-blue.svg)](./LICENSE)

## Why?

`grep`-like commands on unix-like OSes are great. In fact, a search using `grep` is faster than the search on your IDE when there is no code index. 

But, one key pain? A `grep -r` scans _all_ files and not just _source code_ files, making grep-like commands slow. Sometimes, *very* slow. So, there is a need for a command that can scan *only* source code files.

## What?

**scgrep**, which stands for '**s**ource **c**ode **grep**', is a lightweight CLI tool that wraps *your* system's `grep` command and runs it only against source code files.

All grep flags and patterns are passed through as-is to the underlying grep command.

Internally, it fans out across multiple goroutines for parallel directory scanning, making it faster than a plain `grep -r` on large codebases.

It traverses directory trees with source-code awareness — scanning files by known extensions (`.go`, `.java`, `.py`, `.yml`, etc.) and known filenames (`Dockerfile`, `postinst`, etc.), while skipping irrelevant directories (`.git`, `node_modules`, `build`, `.gradle` etc.)

In a Git directory, `scgrep` respects the `.gitignore` file. 

## How to install?

1. Install Go version at least **1.25**
    * See: [Go installation instructions](https://go.dev/doc/install)
2. Run command:
   ```bash
   go install github.com/m-manu/scgrep@latest
   ```
3. Add following line in your `.bashrc`/`.zshrc` file:
   ```bash
   export PATH="$PATH:$HOME/go/bin"
   ```

## Examples of usage

```shell
scgrep --color "LinkedHashSet" .
```

## Which files does it ignore?

`scgrep` command traverses file tree with source code awareness in following ways:

1. Scans for files with known source code and configuration file extensions (case-insensitive)
    * e.g.`.java`, `.go`, `.py`, `.yml` etc.
    * see [full list](config/allowed_file_extensions.txt)
2. Scans for files with certain names (case-sensitive)
    * e.g. `postinst`, `Dockerfile` etc.
    * see [full list](config/allowed_file_names.txt)
3. Skips scanning certain directories (case-sensitive)
    * e.g. `.git`, `.idea`, `.gradle` etc.
    * see [full list](config/ignored_directories.txt)
4. Skips scanning certain directories with specific peer files (case-sensitive)
    * e.g. skip `build` sub-directory when `build.gradle` exists in the same directory etc.
    * see [full list](config/ignored_directories_with_peer_file_names.json)

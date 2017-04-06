# Overview

Slim is a Go tool that lists only those packages affected by a git diff.

# Install

```sh
$ go get -u github.com/kevin-cantwell/slim/cmd/...
```

# Usage

```sh
$ slim -diff HEAD ./...
./cmd/slim
```

# Algorithm

At a high-level: Slim evaluates the diff flag as a git diff and discovers packages in the current project that might
be impacted by each file change. The output can be passed directly into go test. 

At a low level:
* Any path with a change to *_test.go or *.go files will be listed (not including files prefixed with "." or "_").
* Any package with buildable go files which depends on the above will be listed.
* If a file inside a testdata directory has changed, all its parent directories that contain *_test.go files will be 
listed. This is a conservative assumption that a test may depend on testdata.
# Overview

Slim is a Go tool that lists only those packages affected by source control changes. In other words, if you are working on a ginormous monorepo, this ought to speed up your build times tremendously without sacrificing proper testing techniques.

# Install

```sh
$ go get -u github.com/kevin-cantwell/slim/cmd/...
```

# Usage

```sh
Usage of slim:
  slim [-debug] [-diff <diff>] [<packages>]
Options:
  -debug
      Verbose output.
  -diff string
      The git commit pattern to diff by. E.g.: 'HEAD', or '<commit>...<commit>' (default "HEAD")
```

Where `<packages>` is the standard go packages pattern (see `go help list`).

# Algorithm

At a high-level: Slim evaluates the diff flag as a git diff and discovers packages in the current project that might
be impacted by each file change. The output can be passed directly into go test. 

At a low level:
* Any path with a change to `*_test.go` or `*.go` files will be listed (not including files prefixed with `"."` or `"_"`).
* Any package with buildable go files which depends on the above will be listed.
* If changed file resides inside a testdata directory, all the parent directories that contain `*_test.go` files will be 
listed. This is a conservative assumption that a test may depend on testdata directories adjacent to or beneath it.
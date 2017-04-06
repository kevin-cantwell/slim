package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/build"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

var (
	diff  = flag.String("diff", "HEAD", "The git commit pattern to diff by. E.g.: 'HEAD', or '<commit>...<commit>'")
	debug = flag.Bool("debug", false, "Verbose output.")
)

const (
	sep              = string(filepath.Separator)
	testdataPattern1 = "testdata" + sep
	testdataPattern2 = sep + "testdata" + sep
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\t%s [-debug] [-diff <diff>] [<packages>]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	diffs := gitAllDiffs(*diff)
	debugDo(func() {
		fmt.Println("--- git diffs ---")
		for _, file := range diffs.SortedSlice() {
			fmt.Println(file)
		}
		fmt.Println()
	})

	packages := goList(flag.Args())

	impacted := pathsImpacted(packages, diffs)

	debugDo(func() {
		fmt.Println("--- paths impacted ---")
		for _, path := range impacted.SortedSlice() {
			fmt.Println("." + sep + path)
		}
		fmt.Println()
	})

	removePathsWithoutBuildableGoFiles(impacted)

	debugDo(func() {
		fmt.Println("--- buildable paths impacted ---")
	})

	for _, path := range impacted.SortedSlice() {
		fmt.Println("." + sep + path)
	}
}

type StringSet map[string]bool

func (set StringSet) Add(vals ...string) {
	for _, val := range vals {
		set[val] = true
	}
}

func (set StringSet) Exists(val string) bool {
	return set[val]
}

func (set StringSet) Del(val string) {
	delete(set, val)
}

func (set StringSet) SortedSlice() []string {
	slice := make([]string, len(set))
	var i int
	for val := range set {
		slice[i] = val
		i++
	}
	sort.Strings(slice)
	return slice
}

func (set StringSet) Merge(o StringSet) {
	for val := range o {
		set.Add(val)
	}
}

func removePathsWithoutBuildableGoFiles(paths StringSet) {
	for path := range paths {
		infos, err := ioutil.ReadDir(path)
		if err != nil {
			paths.Del(path)
		}
		var hasBuildableGoFiles bool
		for _, info := range infos {
			if strings.HasSuffix(info.Name(), ".go") {
				hasBuildableGoFiles = true
				break
			}
		}
		if !hasBuildableGoFiles {
			paths.Del(path)
		}
	}
}

func hasTestFiles(path string) bool {
	infos, err := ioutil.ReadDir(path)
	check(err)
	for _, info := range infos {
		if strings.HasSuffix(info.Name(), "_test.go") && info.Name()[0] != '.' && info.Name()[0] != '_' {
			return true
		}
	}
	return false
}

func pathsImpacted(packages []Package, diffs StringSet) StringSet {
	// ie: locations that need testing
	impactedPaths := StringSet{}
	// ie: go code that's filed
	alteredPaths := StringSet{}

	projectDir := gitRoot()
	for file := range diffs {
		/*
			The following is a set of rules for how to handle different types of diffs:
			- If a file is ignored by the go tool, then we ignore it too.
			- If a file is inside a testdata directory, then we mark all ancestors of testdata as impacted
			- If a file is a test file, then of course we mark that path as impacted
			- If a file is a .go file, then we mark that path as impacted AND altered
		*/
		basename := filepath.Base(file)
		dir := filepath.Dir(file)
		switch {
		case strings.HasPrefix(basename, "."):
			// The go tool ignores "dot" files and so shall we
			continue
		case strings.HasPrefix(basename, "_"):
			// The go tool ignores files with "_" prefixes and so shall we
			continue
		case strings.HasSuffix(basename, "_test.go"):
			// Good to ".go"! Get it? It's funny cuz it's Go...
			impactedPaths.Add(dir)
			continue
		case strings.HasPrefix(dir, testdataPattern1):
			// Then the project root needs testing (eg: testdata/foo/bar.txt)
			if hasTestFiles(".") {
				impactedPaths.Add(".")
			}
			continue
		case strings.Contains(dir, testdataPattern2):
			// Changes to "testdata" directories impact tests in the parent directory and all ancestor dirs.
			// (eg: foo/testdata/bar.txt)
			for parentDir := dir[:strings.Index(dir, testdataPattern2)]; parentDir != "."; parentDir = filepath.Dir(parentDir) {
				if hasTestFiles(parentDir) {
					impactedPaths.Add(parentDir)
				}
			}
			continue
		case strings.HasSuffix(basename, ".go"):
			impactedPaths.Add(dir)
			alteredPaths.Add(dir)
			continue
		}
	}

	for _, pkg := range packages {
		func() {
			// Check if this package itself was altered
			pkgRelativePath, err := filepath.Rel(projectDir, pkg.Dir)
			check(err)

			if alteredPaths.Exists(pkgRelativePath) {
				impactedPaths.Add(pkgRelativePath)
				return
			}

			// Check the package's dependencies to see if any were altered
			for _, dep := range pkg.Deps {
				buildPkg, err := build.Import(dep, projectDir, build.FindOnly)
				check(err)

				depRelativePath, err := filepath.Rel(projectDir, buildPkg.Dir)
				check(err)

				if alteredPaths.Exists(depRelativePath) {
					impactedPaths.Add(depRelativePath)
					return
				}
			}

			// Check the package's test imports to see if any were altered
			for _, dep := range pkg.TestImports {
				buildPkg, err := build.Import(dep, projectDir, build.FindOnly)
				check(err)

				depRelativePath, err := filepath.Rel(projectDir, buildPkg.Dir)
				check(err)

				if alteredPaths.Exists(depRelativePath) {
					impactedPaths.Add(depRelativePath)
					return
				}
			}

			// Check the package's external test imports to see if any were altered
			for _, dep := range pkg.XTestImports {
				buildPkg, err := build.Import(dep, projectDir, build.FindOnly)
				check(err)

				depRelativePath, err := filepath.Rel(projectDir, buildPkg.Dir)
				check(err)

				if alteredPaths.Exists(depRelativePath) {
					impactedPaths.Add(depRelativePath)
					return
				}
			}
		}()
	}

	return impactedPaths
}

func debugDo(fn func()) {
	if *debug {
		fn()
	}
}

func shell(executable string, args ...string) []byte {
	cmd := exec.Command(executable, args...)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	check(err)
	return output
}

func check(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func failf(err string) {
	fmt.Println(err)
	os.Exit(1)
}

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "\t")
	return enc.Encode(v)
}

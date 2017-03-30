package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/timehop/whois"
)

var (
	_ = whois.Client{}

	diff = flag.String("diff", "HEAD", "The git commit pattern to diff by. E.g.: 'HEAD', or '<commit>...<commit>'")
)

func main() {
	flag.Parse()

	// packages := os.Args[1]
	// fmt.Println("\nargs:", flag.Args())

	// fmt.Println("\nfile changes:")
	changes := fileChanges(*diff)
	// for _, change := range changes {
	// 	fmt.Println(change)
	// }

	// fmt.Println("\ngo list:")
	packages := goList([]string{"./..."})
	// for _, pkg := range packages {
	// 	fmt.Println(pkg.ImportPath)
	// }

	// fmt.Println("\npackages under test:")
	for _, importPath := range importPathsUnderTest(packages, changes) {
		fmt.Println(importPath)
	}

	// printJSON(pkg)
}

const sep = string(filepath.Separator)

func importPathsUnderTest(packages []Package, changes []string) []string {
	gitRootDir := gitRoot()

	// eg: {"github.com/timehop/whois": "/Users/foo/go/src/github.com/timehop/whois"}
	changedImportPaths := map[string]bool{}
	// gopathSrcPrefix := os.Getenv("GOPATH") + sep + "src" + sep
	for _, change := range changes {
		buildPackage, err := build.ImportDir(gitRootDir+sep+filepath.Dir(change), build.FindOnly)
		check(err)
		changedImportPaths[buildPackage.ImportPath] = true
		// changeDirPath := gitRootDir + sep + filepath.Dir(change)
		// if !strings.HasPrefix(changeDirPath, gopathSrcPrefix) {
		//  failf("Change is not rooted in GOPATH: %v", gitRootDir+sep+change)
		// }
		//   changedImportPaths[]
	}

	dependents := map[string]bool{}
	for _, pkg := range packages {
		if changedImportPaths[pkg.ImportPath] {
			dependents[pkg.ImportPath] = true
			continue
		}
		for _, dep := range pkg.Deps {
			if changedImportPaths[dep] {
				dependents[pkg.ImportPath] = true
				continue
			}
		}
	}
	var underTest []string
	for importPath := range dependents {
		underTest = append(underTest, importPath)
	}
	sort.Strings(underTest)
	return underTest
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

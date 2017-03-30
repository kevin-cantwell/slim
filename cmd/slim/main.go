package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kevin-cantwell/slim"
)

var (
	// To be merged with the output of:
	// git status --short --untracked-files --porcelain | grep "??" | cut -c 4-
	diff = flag.String("diff", "HEAD", "The git diff pattern, plus all untracked files. E.g.: 'HEAD', '<commit>...<commit>'")
)

func main() {
	flag.Parse()

	// packages := os.Args[1]
	fmt.Println("Args:", flag.Args())

	difference, err := slim.Difference(*diff, os.Stderr)
	fmt.Println(difference, err)

	// wd, _ := os.Getwd()
	// pkg, err := build.Import("github.com/timehop/whois", wd, build.ImportComment)
	// if err != nil {
	// 	panic(err)
	// }

	// printJSON(pkg)
}

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "\t")
	return enc.Encode(v)
}

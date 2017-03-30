package main

import (
	"bytes"
	"encoding/json"
	"io"
)

type Package struct {
	Dir        string
	ImportPath string
	Deps       []string
}

func goList(args []string) []Package {
	output := shell("go", append([]string{"list", "-json"}, args...)...)
	buf := bytes.NewBuffer(output)
	dec := json.NewDecoder(buf)
	var packages []Package
	for {
		var pkg Package
		if err := dec.Decode(&pkg); err != nil {
			if err == io.EOF {
				break
			}
			check(err)
		}
		packages = append(packages, pkg)
	}
	return packages
}

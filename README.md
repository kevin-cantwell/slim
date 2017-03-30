# Overview

Slim is a Go tool for only testing code that has changed.

* Test a *_test.go file if it has changed.
* Test a file if a non-test *.go file in the same package has changed
* Test a file if a non-test *.go file from a package that it depends upon (recursively) has changed.
* Test a file if any file under ./testdata has changed
* Test a file if any non-go file under ../ has changed that doesn't start with "." or "_"

$ slim test ./...

# All touched files that matter:
all_touched=`slim diff <commit> | grep -v ".*" -v "_*"`
all_touched_go=`cat all_touched | grep -e "*.go" 

We pull out all the go files from the above, parse out the imports (or  )
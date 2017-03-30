/*
  For go list structs, see:
  https://golang.org/pkg/go/build


*/
package slim

import (
	"bytes"
	"io"
	"os/exec"
	"strings"
)

/*
  Difference determines which files in the project have changed, according to Git.
  Returns a slice of filenames relative to the project root and any error from
  executing the git command. The commitComparison behaves similarly to git diff:

    ""
      All changes in your working tree, plus untracked files, plus cached files.
        git status --short --untracked-files --porcelain

    "<commit>"
      All changes from the previous form, plus changes relative to the named <commit>.
        git diff --name-only <commit>

    "<commit> <commit>"
      All changes between two arbitrary <commit>.
        git diff --name-only <commit> <commit>

    "<commit>..<commit>"
      This is synonymous to the previous form. If <commit> on one side is omitted, it will
      have the same effect as using HEAD instead.
        git diff --name-only <commit>..<commit>

    "<commit>...<commit>"
      All changes on the branch containing and up to the second <commit>. If <commit> on
      one side is omitted, it will have the same effect as using HEAD instead.
        git diff --name-only <commit>...<commit>

  Any errors written by git will be reported to stderr.
*/
func Difference(commitComparison string, stderr io.Writer) ([]string, error) {
	commitComparison = strings.TrimSpace(commitComparison)

	if commitComparison == "" {
		return gitNew(false, stderr)
	}
	diffs, err := gitDiff(commitComparison, stderr)
	if err != nil {
		return nil, err
	}
	// If it's an explicit comparison, we don't include new files
	if strings.ContainsAny(commitComparison, " .") { // "sha1 sha2", "sha1..sha2", or "sha1...sha2"
		return diffs, nil
	}

	untracked, err := gitNew(true, stderr)
	if err != nil {
		return nil, err
	}
	return append(diffs, untracked...), nil
}

// git status --short --untracked-files --porcelain
func gitNew(untrackedOnly bool, stderr io.Writer) ([]string, error) {
	/*
	  The output will look like this, where the first two characters
	  indicate the status, followed by a space, followed by the filename
	  relative to the project root:

	   D README.md
	  M  circle.yml
	  ?? foo.bar
	  ?? thjson/bar/baz/biz.txt
	*/
	cmd := exec.Command("git", "status", "--short", "--untracked-files", "--porcelain")
	cmd.Stderr = stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var filenames []string
	files := bytes.Split(output, []byte{'\n'})
	for _, file := range files {
		if untrackedOnly && string(file[0:2]) != "??" {
			continue
		}
		filenames = append(filenames, string(file[3:]))
	}
	return filenames, nil
}

// git diff --name-only <commitPattern>
func gitDiff(commitPattern string, stderr io.Writer) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", commitPattern)
	cmd.Stderr = stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	files := bytes.Split(output, []byte{'\n'})
	filenames := make([]string, len(files))
	for i := 0; i < len(files); i++ {
		filenames[i] = string(files[i])
	}
	return filenames, nil
}

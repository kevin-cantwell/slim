package main

import (
	"bytes"
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

  Any errors written by git will be reported to stderr. May return duplicates.
*/
func fileChanges(commitComparison string) []string {
	commitComparison = strings.TrimSpace(commitComparison)

	// Then just return everything from status
	// if commitComparison == "" {
	// 	return gitStatus(false)
	// }

	diffs := gitDiff(commitComparison)

	// If it's an explicit comparison, we don't include new files
	if strings.ContainsAny(commitComparison, " .") { // "sha1 sha2", "sha1..sha2", or "sha1...sha2"
		return diffs
	}

	// If it's a single commit comparison (ie: HEAD), then append untracked files
	return append(diffs, gitStatus(false)...)
}

// git status --short --untracked-files --porcelain
func gitStatus(onlyUntracked bool) []string {
	/*
	   The output will look like this, where the first two characters
	   indicate the status, followed by a space, followed by the filename
	   relative to the project root:

	    D README.md
	   M  circle.yml
	   ?? foo.bar
	   ?? thjson/bar/baz/biz.txt
	*/
	output := shell("git", "status", "--short", "--untracked-files", "--porcelain")

	var filenames []string
	for _, file := range bytes.Split(output, []byte{'\n'}) {
		if len(file) == 0 {
			continue
		}
		if onlyUntracked && string(file[0:2]) != "??" {
			continue
		}

		file = file[3:]                                                  // Strip the XY_ prefix
		if split := bytes.Split(file, []byte(" -> ")); len(split) == 2 { // Handle file renames
			filenames = append(filenames, string(split[0]))
			file = split[1]
		}
		filenames = append(filenames, string(file))
	}
	return filenames
}

// git diff --name-only <commitPattern>
func gitDiff(commitPattern string) []string {
	output := shell("git", "diff", "--name-only", commitPattern)

	var filenames []string
	for _, file := range bytes.Split(output, []byte{'\n'}) {
		if len(file) == 0 {
			continue
		}
		filenames = append(filenames, string(file))
	}
	return filenames
}

func gitRoot() string {
	return strings.TrimSpace(string(shell("git", "rev-parse", "--show-toplevel")))
}

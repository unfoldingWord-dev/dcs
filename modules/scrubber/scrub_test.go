// Copyright 2019 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

/*** DCS Customizations - Tests for scrubbing repos ***/

package scrubber_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/scrubber"

	"github.com/stretchr/testify/assert"
)

// The repos to be tested and if they show throw an error (return nil if no error)
var TestingRepos = map[string]bool{
	"all_json_files":            false,
	"bad_json_file":             true,
	"multiple_sensitive_fields": false,
	"no_json_files":             false,
	"no_sensitive_data":         false,
}

func TestScrubJsonFiles(t *testing.T) {
	myDir, _ := os.Getwd()
	testFilesDir := path.Join(myDir, "scrub_test_files")
	tempDir, _ := ioutil.TempDir(os.TempDir(), "scrub_test")
	for repoName, throwsError := range TestingRepos {
		repoDir := path.Join(tempDir, repoName)
		fmt.Println("Copying ", path.Join(testFilesDir, repoName), "==>", repoDir)
		CopyDir(path.Join(testFilesDir, repoName), repoDir)
		git.InitRepository(repoDir, false)
		git.AddChanges(repoDir, true)
		git.CommitChanges(repoDir, git.CommitChangesOptions{
			Committer: &git.Signature{
				Name:  "John Smith",
				Email: "john@smith.com",
				When:  time.Now(),
			},
			Author: &git.Signature{
				Name:  "John Smith",
				Email: "john@smith.com",
				When:  time.Now(),
			},
			Message: "Initial Commit",
		})
		if throwsError {
			assert.NotNil(t, scrubber.ScrubJSONFiles(repoDir))
		} else {
			assert.Nil(t, scrubber.ScrubJSONFiles(repoDir))
		}
	}
}

// The below code to copy directories was retrieved by Richard Mahn from https://gist.github.com/m4ng0squ4sh/92462b38df26839a3ca324697c8cba04

// CopyFile copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file. The file mode will be copied from the source and
// the copied data is synced/flushed to stable storage.
func CopyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		if e := out.Close(); e != nil {
			err = e
		}
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return
	}

	err = out.Sync()
	if err != nil {
		return
	}

	si, err := os.Stat(src)
	if err != nil {
		return
	}
	err = os.Chmod(dst, si.Mode())
	if err != nil {
		return
	}

	return
}

// CopyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination directory must *not* exist.
// Symlinks are ignored and skipped.
func CopyDir(src string, dst string) (err error) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !si.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	_, err = os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	if err == nil {
		return fmt.Errorf("destination already exists")
	}

	err = os.MkdirAll(dst, si.Mode())
	if err != nil {
		return
	}

	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = CopyDir(srcPath, dstPath)
			if err != nil {
				return
			}
		} else {
			// Skip symlinks.
			if entry.Mode()&os.ModeSymlink != 0 {
				continue
			}

			err = CopyFile(srcPath, dstPath)
			if err != nil {
				return
			}
		}
	}
	return
}

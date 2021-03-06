// Copyright (C) 2014 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package osutil_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/syncthing/syncthing/lib/fs"
	"github.com/syncthing/syncthing/lib/osutil"
)

func TestIsDeleted(t *testing.T) {
	type tc struct {
		path  string
		isDel bool
	}
	cases := []tc{
		{"del", true},
		{"del.file", false},
		{"del/del", true},
		{"file", false},
		{"linkToFile", false},
		{"linkToDel", false},
		{"linkToDir", false},
		{"linkToDir/file", true},
		{"file/behindFile", true},
		{"dir", false},
		{"dir.file", false},
		{"dir/file", false},
		{"dir/del", true},
		{"dir/del/del", true},
		{"del/del/del", true},
	}

	testFs := fs.NewFilesystem(fs.FilesystemTypeBasic, "testdata")

	testFs.MkdirAll("dir", 0777)
	for _, f := range []string{"file", "del.file", "dir.file", "dir/file"} {
		fd, err := testFs.Create(f)
		if err != nil {
			t.Fatal(err)
		}
		fd.Close()
	}
	if runtime.GOOS != "windows" {
		// Can't create unreadable dir on windows
		testFs.MkdirAll("inacc", 0777)
		if err := testFs.Chmod("inacc", 0000); err == nil {
			if _, err := testFs.Lstat("inacc/file"); fs.IsPermission(err) {
				// May fail e.g. if tests are run as root -> just skip
				cases = append(cases, tc{"inacc", false}, tc{"inacc/file", false})
			}
		}
	}
	for _, n := range []string{"Dir", "File", "Del"} {
		if err := osutil.DebugSymlinkForTestsOnly(filepath.Join(testFs.URI(), strings.ToLower(n)), filepath.Join(testFs.URI(), "linkTo"+n)); err != nil {
			if runtime.GOOS == "windows" {
				t.Skip("Symlinks aren't working")
			}
			t.Fatal(err)
		}
	}

	for _, c := range cases {
		if osutil.IsDeleted(testFs, c.path) != c.isDel {
			t.Errorf("IsDeleted(%v) != %v", c.path, c.isDel)
		}
	}

	testFs.Chmod("inacc", 0777)
	os.RemoveAll("testdata")
}

func TestRenameOrCopy(t *testing.T) {
	mustTempDir := func() string {
		t.Helper()
		tmpDir, err := ioutil.TempDir("", "")
		if err != nil {
			t.Fatal(err)
		}
		return tmpDir
	}
	sameFs := fs.NewFilesystem(fs.FilesystemTypeBasic, mustTempDir())
	tests := []struct {
		src  fs.Filesystem
		dst  fs.Filesystem
		file string
	}{
		{
			src:  sameFs,
			dst:  sameFs,
			file: "file",
		},
		{
			src:  fs.NewFilesystem(fs.FilesystemTypeBasic, mustTempDir()),
			dst:  fs.NewFilesystem(fs.FilesystemTypeBasic, mustTempDir()),
			file: "file",
		},
		{
			src:  fs.NewFilesystem(fs.FilesystemTypeFake, `fake://fake/?files=1&seed=42`),
			dst:  fs.NewFilesystem(fs.FilesystemTypeBasic, mustTempDir()),
			file: osutil.NativeFilename(`05/7a/4d52f284145b9fe8`),
		},
	}

	for _, test := range tests {
		content := test.src.URI()
		if _, err := test.src.Lstat(test.file); err != nil {
			if !fs.IsNotExist(err) {
				t.Fatal(err)
			}
			if fd, err := test.src.Create(test.file); err != nil {
				t.Fatal(err)
			} else {
				if _, err := fd.Write([]byte(test.src.URI())); err != nil {
					t.Fatal(err)
				}
				_ = fd.Close()
			}
		} else {
			fd, err := test.src.Open(test.file)
			if err != nil {
				t.Fatal(err)
			}
			buf, err := ioutil.ReadAll(fd)
			if err != nil {
				t.Fatal(err)
			}
			_ = fd.Close()
			content = string(buf)
		}

		err := osutil.RenameOrCopy(test.src, test.dst, test.file, "new")
		if err != nil {
			t.Fatal(err)
		}

		if fd, err := test.dst.Open("new"); err != nil {
			t.Fatal(err)
		} else {
			if buf, err := ioutil.ReadAll(fd); err != nil {
				t.Fatal(err)
			} else if string(buf) != content {
				t.Fatalf("expected %s got %s", content, string(buf))
			}
		}
	}
}

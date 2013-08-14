package main

import (
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var (
	zerotime time.Time = time.Time{}
	before             = time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)
	same               = time.Date(2000, 1, 1, 1, 1, 1, 1000, time.UTC)
	after              = time.Date(2000, 1, 1, 1, 1, 2, 0, time.UTC)

	table = map[string][2][]File{
		// "Paths": {{
		// 	{"/a/b", before, NewFake("")},
		// 	{"/b", after, NewFake("")},
		// }, {
		// 	{"/a", after, NewFake("")},
		// 	{"/c", before, NewFake("")},
		// }},
		// "Timestamps": {{
		// 	{"/a", before, NewFake("before")},
		// 	{"/b", after, NewFake("after")},
		// }, {
		// 	{"/a", after, NewFake("after")},
		// 	{"/b", before, NewFake("before")},
		// }},
		"Zerotime": {{ //TODO support zerotime
		//	{"/b", after, NewFake("")},
		}, {
		//	{"/b", zerotime, NewFake("")},
		}},
	}
)

func TestLocal(t *testing.T) {
	for scenario, fss := range table {
		// Write Files to Local
		base, rm := TempDir()
		defer rm()

		for _, file := range append(fss[0], fss[1]...) {
			file.Url.Path = filepath.Join(base, file.Url.Path)
			err := Local(file)
			if err != nil {
				t.Fatal(err)
			}
		}

		//Compare Local Files with expected
		expected := expectedBehaviour(NewTestFs(fss[1]),
			NewTestFs(fss[0]))
		actual, err := LocalTestFs(base)
		if err != nil {
			t.Fatal(err)
		}
		printErrors("", scenario, expected, NewTestFs(actual), t)
	}

}

func LocalTestFs(basepath string) (files []File, err error) {
	err = filepath.Walk(basepath,
		func(path string, info os.FileInfo, inerr error) (err error) {
			if inerr != nil {
				return inerr
			}
			if info.IsDir() {
				return
			}
			file, err := os.Open(path)
			if err != nil {
				return
			}
			content, err := ioutil.ReadAll(file)
			mtime := info.ModTime()
			if mtime.Equal(unixZerotime) {
				mtime = zerotime
			}
			files = append(files,
				File{
					Url:      &url.URL{Path: path[len(basepath):]},
					Mtime:    info.ModTime(),
					FileFunc: NewFake(string(content)),
				})
			return
		})
	return
}

func expectedBehaviour(remote, local testFS) (expected testFS) {
	expected = make(testFS)
	//TODO: Atime behaviour

	for path, file := range local {
		expected[path] = file
	}
	for path, file := range remote {
		// Many filesystems do not support sub seconds timestamps
		mtime := removeNanoseconds(file.Mtime)
		lmtime := removeNanoseconds(local[path].Mtime)
		if mtime.After(lmtime) {
			expected[path] = file
		} else {
			expected[path] = local[path]
		}
	}
	return
}

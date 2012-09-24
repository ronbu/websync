package main

import (
	"net/http/httptest"
	// "errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testFS map[string]File

func NewTestFs(files []File) (res testFS) {
	res = make(testFS)
	for _, file := range files {
		res[file.Path] = file
	}
	return
}

type handlerAndlistFunc struct {
	handler  func(testFS, *testing.T) http.Handler
	listFunc remoteFunc
}

type FakeReadCloser struct {
	*strings.Reader
	closed bool
}

func (d FakeReadCloser) Close() error {
	d.closed = true
	return nil
}

func NewFake(content string) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		return FakeReadCloser{Reader: strings.NewReader(content)}, nil
	}
}

var (
	zerotime time.Time = time.Time{}
	before             = time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)
	same               = time.Date(2000, 1, 1, 1, 1, 1, 1000, time.UTC)
	after              = time.Date(2000, 1, 1, 1, 1, 2, 0, time.UTC)

	table = map[string][2][]File{
		"Paths": {{
			{"/a/b", before, NewFake("")},
			{"/b", after, NewFake("")},
		}, {
			{"/a/b", after, NewFake("")},
			{"/c", before, NewFake("")},
		}},
		"Comparing Timestamps": {{
			{"/a", before, NewFake("")},
			{"/b", after, NewFake("")},
		}, {
			{"/a", after, NewFake("")},
			{"/b", before, NewFake("")},
		}},
		"Zerotime": {{ //TODO support zerotime
		//	{"a", zerotime, NewFake("")},
		//	{"b", after, NewFake("")},
		}, {
		// {"a", after, NewFake("")},
		// {"b", zerotime, NewFake("")},
		}},
	}

	handlers = map[string]handlerAndlistFunc{
		"webdav": {handler: NewDavHandler, listFunc: Dav},
	}
)

func TestEndToEnd(t *testing.T) {
	username, password := "user", "pw"
	for scenario, fss := range table {
		for handlername, handlerAndlistFunc := range handlers {
			// Write Files to Local
			ch := make(chan bool, 0)
			base := TempDir(t, "TestLocal", ch)

			for _, file := range fss[1] {
				file.Path = filepath.Join(base, file.Path)
				err := Local(file)
				if err != nil {
					t.Fatal(err)
				}
			}

			// Sync with Fake Remote
			handler := handlerAndlistFunc.handler(NewTestFs(fss[0]), t)
			server := httptest.NewServer(handler)
			serverurl, _ := url.Parse(server.URL)

			if err := Sync(base, handlerAndlistFunc.listFunc,
				*serverurl, DefaultClient, username, password); err != nil {
				t.Fatal(err)
			}

			//Compare Local Files with expected
			expected := expectedBehaviour(NewTestFs(fss[0]), NewTestFs(fss[1]))
			actual, err := LocalTestFs(base)
			if err != nil {
				t.Fatal(err)
			}
			printErrors(handlername, scenario, expected, NewTestFs(actual), t)
			ch <- true
		}
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
					Path:     path[len(basepath):],
					Mtime:    info.ModTime(),
					FileFunc: NewFake(string(content)),
				})
			return
		})
	return
}

func printErrors(handlername, scenario string,
	expected, actual testFS, t *testing.T) {
	//t.Logf("%v\n%v\n", expected, actual)
	errs := findErrors(expected, actual)
	if len(errs) > 0 {
		t.Logf("Handler: '%s' in Scenario: '%s'\n",
			handlername, scenario)
		//t.Logf("Actual: %#v\n", actual)
		t.Fail()
	}
	for _, error := range errs {
		t.Log(error)
	}
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
		if mtime.After(lmtime) ||
			file.Mtime.Equal(zerotime) ||
			local[path].Mtime.Equal(zerotime) {
			expected[path] = file
		} else {
			expected[path] = local[path]
		}
	}
	return
}

func findErrors(expected, actual testFS) (errs []string) {
	for path, file := range expected {
		afile, ok := actual[path]
		if !ok {
			errs = append(errs, "Path: '"+path+"' is missing")
			continue
		}
		content, _ := file.ReadAll()
		acontent, err := afile.ReadAll()
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		helper := func(field string, a, e interface{}) {
			errs = append(errs,
				fmt.Sprintf("%s: field '%s' %v should be %v", path, field, a, e))
		}
		if !afile.Mtime.Equal(file.Mtime) {
			helper("mtime", afile.Mtime, file.Mtime)
		} else if string(acontent) != string(content) {
			helper("content", string(acontent), string(content))
		}
	}
	return
}

func TempDir(t *testing.T, prefix string, remove chan bool) (path string) {
	path, err := ioutil.TempDir("", prefix)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = <-remove
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
	}()
	return path
}

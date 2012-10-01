package main

import (
	// "errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
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
	handlers = map[string]handlerAndlistFunc{
		"webdav": {handler: NewDavHandler, listFunc: Dav},
	}
)

/*func TestEndToEnd(t *testing.T) {
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

}*/

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
				fmt.Sprintf("%s: field '%s' %v should be %v",
					path, field, a, e))
		}
		if !afile.Mtime.Equal(file.Mtime) {
			helper("mtime", afile.Mtime, file.Mtime)
		} else if string(acontent) != string(content) {
			helper("content", string(acontent), string(content))
		}
	}
	return
}

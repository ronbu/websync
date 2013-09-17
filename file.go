package main

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ReadFn func() (rc io.ReadCloser, err error)
type File struct {
	path  string
	Url   url.URL
	Mtime time.Time
	Read  ReadFn
}

func NewFile(base File, path string, uri *url.URL, mtime *time.Time, f ReadFn) File {
	if uri != nil {
		base.Url = *uri
	}
	if mtime != nil {
		base.Mtime = *mtime
	}
	if f != nil {
		base.Read = f
	}
	return base.Append(path)
}

func (f File) Path() string {
	return f.path
}

func (f *File) Append(p string) File {
	if len(p) > 0 && p[0] == '/' && len(f.path) > 0 {
		p = p[1:]
	}
	f.path += p
	return *f
}

func (f *File) FromString(s string) File {
	f.FromReader(strings.NewReader(s))
	return *f
}

func (f *File) FromReader(r io.Reader) File {
	f.Read = func() (rc io.ReadCloser, err error) {
		return fakeCloser{r}, nil
	}
	return *f
}

type fakeCloser struct{ io.Reader }

func (f fakeCloser) Close() (err error) { return }

func (f *File) FromUrl(u string) File {
	f.Read = func() (rc io.ReadCloser, err error) {
		req, err := http.NewRequest("GET", u, strings.NewReader(""))
		if err != nil {
			return nil, err
		}
		f.FromRequest(req)
		return f.Read()
	}
	return *f
}

func (f *File) FromRequest(req *http.Request) File {
	f.Read = func() (rc io.ReadCloser, err error) {
		resp, err := HClient.Do(req)
		if err != nil {
			return nil, err
		}
		sc := resp.StatusCode
		if sc >= 200 && sc < 300 {
			return resp.Body, nil
		} else {
			return nil, errors.New(
				req.URL.String() + ": " + resp.Status)
		}
	}
	return *f
}

func (f File) ReadAll() (content []byte, err error) {
	reader, err := f.Read()
	if err != nil {
		return
	}
	return ioutil.ReadAll(reader)
}
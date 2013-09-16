package main

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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

func NewFile(base File, path string, uri *url.URL, f ReadFn) File {
	if uri != nil {
		base.Url = *uri
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
	if len(p) > 0 && p[0] == '/' {
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

func Sync(from, to string, lookup LookupFn) (chan File, chan error) {
	return injectableSync(from, to, lookup, Local)
}

func injectableSync(from, to string, lookup LookupFn, writeFile func(File) error) (
	chan File, chan error) {

	files := make(chan File)
	errs := make(chan error)

	fromUri, err := url.Parse(from)
	if err != nil {
		errs <- err
		return nil, errs
	}

	if to != "" && to[len(to)-1] != '/' {
		to += "/"
	}

	go func() {
		recursiveSync((&File{Url: *fromUri}).Append(to), files, errs, lookup, writeFile)
		close(files)
		close(errs)
	}()
	return files, skipNil(errs)
}

func recursiveSync(f File, files chan File, errs chan error,
	lookup LookupFn, writeFile func(File) error) {

	indexFn, err := lookup(f)
	errs <- err
	if indexFn == nil {
		errs <- errors.New("Not Supported: " + f.Url.String())
		return
	}

	hfiles := make(chan File)
	finish := make(chan bool)
	go func() {
		indexFn(f, hfiles, errs)
		finish <- true
	}()

LOOP:
	for {
		select {
		case <-finish:
			break LOOP
		case f := <-hfiles:
			if f.Read == nil {
				recursiveSync(f, files, errs, lookup, writeFile)
			} else {
				err = writeFile(f)
				if err != nil {
					errs <- err
				} else {
					files <- f
				}
			}
		}
	}
}

func skipNil(in chan error) chan error {
	out := make(chan error, len(in))
	go func() {
		for e := range in {
			if e != nil {
				out <- e
			}
		}
		close(out)
	}()
	return out
}

func Local(file File) (err error) {
	path := file.Path()
	st, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	if st == nil || file.Mtime.After(st.ModTime()) {
		err = os.MkdirAll(filepath.Dir(path), 0777)
		if err != nil {
			return err
		}
		osfile, err := os.Create(path)
		if err != nil {
			return err
		}
		defer osfile.Close()
		r, err := file.Read()
		if err != nil {
			return err
		}
		_, err = io.Copy(osfile, r)
		if err != nil {
			return err
		}
		os.Chtimes(path, file.Mtime, file.Mtime)
	}
	return
}

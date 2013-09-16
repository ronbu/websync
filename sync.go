package main

import (
	"errors"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type ReadFn func() (reader io.ReadCloser, err error)
type File struct {
	Path     string
	Url      url.URL
	Mtime    time.Time
	FileFunc ReadFn
}

func (f File) ReadAll() (content []byte, err error) {
	reader, err := f.FileFunc()
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
		recursiveSync(File{Path: to, Url: *fromUri}, files, errs, lookup, writeFile)
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
			if f.FileFunc == nil {
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
	st, err := os.Stat(file.Path)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	if st == nil || file.Mtime.After(st.ModTime()) {
		err = os.MkdirAll(filepath.Dir(file.Path), 0777)
		if err != nil {
			return err
		}
		osfile, err := os.Create(file.Path)
		if err != nil {
			return err
		}
		defer osfile.Close()
		r, err := file.FileFunc()
		if err != nil {
			return err
		}
		_, err = io.Copy(osfile, r)
		if err != nil {
			return err
		}
		os.Chtimes(file.Path, file.Mtime, file.Mtime)

		osfile.Sync()
	}
	return
}

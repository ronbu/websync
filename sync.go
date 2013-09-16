package main

import (
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
)

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

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

	go func() {
		todos := []File{File{Url: *fromUri}}
		for i := 0; i < len(todos); i++ {
			todo := todos[i]

			indexFn, err := lookup(todo)
			errs <- err
			if indexFn == nil {
				errs <- errors.New("Not Supported: " + todo.Url.String())
				continue
			}

			hfiles := make(chan File)
			finish := make(chan bool)
			go func(f chan bool) {
				indexFn(todo, hfiles, errs)
				f <- true
			}(finish)

		LOOP:
			for {
				select {
				case <-finish:
					break LOOP
				case f := <-hfiles:
					if f.FileFunc == nil {
						todos = append(todos, f)
					} else {
						f.Url.Path = filepath.Join(to, f.Url.Path)
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
		close(files)
		close(errs)
	}()
	return files, skipNil(errs)
}

func skipNil(in chan error) chan error {
	out := make(chan error)
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
	st, err := os.Stat(file.Url.Path)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	if st == nil || file.Mtime.After(st.ModTime()) {
		err = os.MkdirAll(filepath.Dir(file.Url.Path), os.ModeDir|os.ModePerm)
		if err != nil {
			return err
		}
		osfile, err := os.Create(file.Url.Path)
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
		os.Chtimes(file.Url.Path, file.Mtime, file.Mtime)

		osfile.Sync()
	}
	return
}

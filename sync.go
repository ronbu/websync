package main

import (
	"bytes"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func Sync(from, to string) chan File {
	ch := make(chan File)
	if to != "" && to[len(to)-1] != '/' {
		to += "/"
	}
	u, err := url.Parse(from)
	f := File{Path: to}
	if err != nil {
		f.SendErr(ch, &err)
		return ch
	}
	f.Url = *u

	go func() {
		Recursive(f, ch)
		close(ch)
	}()

	out := make(chan File)
	go func() {
		for f := range ch {
			if f.Err == nil {
				var r io.ReadCloser
				if f.Body == nil {
					f, r = HGet(f)
					if f.Err != nil {
						f.SendErr(out, &f.Err)
						continue
					}
				} else {
					r = fakeCloser{bytes.NewReader(f.Body)}
				}
				f.Err = local(f, r)
			}
			out <- f
		}
		close(out)
	}()
	return out
}

func Recursive(f File, ch chan File) {
	recursive(f, ch, Index)
}

func recursive(f File, ch chan File, ifn IndexFn) {
	in := make(chan File)
	finish := make(chan bool)
	go func() {
		ifn(f, in)
		finish <- true
	}()

LOOP:
	for {
		select {
		case <-finish:
			break LOOP
		case f := <-in:
			if f.Err == nil {
				recursive(f, ch, ifn)
			} else {
				if f.Leaf() {
					f.Err = nil
				}
				ch <- f
			}
		}
	}
}

func local(file File, r io.ReadCloser) (err error) {
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
		_, err = io.Copy(osfile, r)
		if err != nil {
			return err
		}
		os.Chtimes(file.Path, file.Mtime, file.Mtime)
	}
	return
}

type fakeCloser struct{ io.Reader }

func newFakeCloser(s string) io.ReadCloser {
	return fakeCloser{strings.NewReader(s)}
}

func (f fakeCloser) Close() error { return nil }

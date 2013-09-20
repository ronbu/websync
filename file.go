package main

import (
	"errors"
	"net/url"
	"time"
)

type File struct {
	Path  string
	Url   url.URL
	Mtime time.Time
	Err   error
	Body  []byte
}

func (f File) SendErr(ch chan File, err *error) {
	if *err != nil {
		f.Err = *err
		ch <- f
		*err = nil
	}
}

var leafErr = errors.New("File is a Leaf!")

func (f File) SetLeaf() File {
	f.Err = leafErr
	return f
}

func (f File) Leaf() bool {
	if f.Err == leafErr {
		return true
	} else {
		return false
	}
}

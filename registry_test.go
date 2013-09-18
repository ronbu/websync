package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func testIndexFn(t *testing.T, ifn IndexFn, f File, expected []File) {
	fs := make(chan File)
	es := make(chan error)
	actual := []File{}
	finish := make(chan bool)
	go func() {
		ifn(f, fs, es)
		finish <- true
	}()
LOOP:
	for {
		select {
		case f := <-fs:
			actual = append(actual, f)
		case e := <-es:
			if e != nil {
				t.Error(e)
			}
		case <-time.After(1 * time.Second):
			t.Error("Index function timed out")
			break LOOP
		case <-finish:
			break LOOP
		}
	}
	for i, ac := range actual {
		exp := expected[i]

		expCont, e := exp.ReadAll()
		if e != nil {
			t.Fatal("Reading Expected File failed: ", e)
		}
		acContent, e := ac.ReadAll()
		if e != nil {
			t.Errorf("Reading %v failed: %v", ac, e)
		}

		if string(expCont) != string(acContent) {
			t.Errorf("Content:\nE: %s\nA: %s", expCont, acContent)
		}
		if !ac.Mtime.Equal(exp.Mtime) {
			t.Errorf("Mtime:\nE: %#s\nA: %#s", exp.Mtime, ac.Mtime)
		}
		if ac.Url.Path != exp.Url.Path {
			t.Errorf("URL:\nE: %s\nA: %s", exp.Url.Path, ac.Url.Path)
		}
	}
}

type testcase struct {
	in  File
	exp []File
}

func testIndex(t *testing.T, urlVar *url.URL, ifn IndexFn,
	h http.Handler, tests []testcase) {

	s := httptest.NewServer(h)
	if urlVar != nil {
		u, _ := url.Parse(s.URL)
		*urlVar = *u
	}
	for _, test := range tests {
		testIndexFn(t, ifn, test.in, test.exp)
	}
	s.Close()
}

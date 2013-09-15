package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testIndexFn(t *testing.T, ifn IndexFn, f File, expected []File) {
	fs := make(chan File)
	es := make(chan error)
	actual := []File{}
	go func() {
		ifn(f, fs, es)
	}()
	for {
		select {
		case f := <-fs:
			actual = append(actual, f)
		case e := <-es:
			if e != nil {
				t.Error(e)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("Index function timed out")
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

		switch {
		case string(expCont) != string(acContent):
			t.Errorf("%s != %s", expCont, acContent)
			fallthrough
		case !(ac.Mtime.Equal(exp.Mtime) && ac.Url == exp.Url):
			t.Errorf("%v != %v", exp, ac)
		}
	}
}

type testCase struct {
	in  File
	exp []File
}

func testIndex(t *testing.T, urlVar *string, ifn IndexFn,
	tests []testCase, tFn func(*testing.T, []File) http.Handler) {
	for _, test := range tests {
		s := httptest.NewServer(tFn(t, test.exp))
		urlVar = &s.URL
		testIndexFn(t, ifn, test.in, test.exp)
		s.Close()
	}
}

package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func testIndexFn(t *testing.T, ifn IndexFn, f File, expected []File) {
	fs := make(chan File)
	actual := []File{}
	finish := make(chan bool)
	go func() {
		ifn(f, fs)
		finish <- true
	}()
LOOP:
	for {
		select {
		case f := <-fs:
			actual = append(actual, f)
		case <-time.After(1 * time.Second):
			t.Error("Index function timed out")
			break LOOP
		case <-finish:
			break LOOP
		}
	}
	for i, ac := range actual {
		exp := expected[i]
		if exp.Url != ac.Url {
			t.Errorf("Url:\nE: %s\nA: %s", exp.Url, ac.Url)
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
	in  string
	exp []string
}

func testIndex(t *testing.T, urlVar *url.URL, ifn IndexFn, h http.Handler,
	tests []testcase) {

	s := httptest.NewServer(h)
	if urlVar != nil {
		u, _ := url.Parse(s.URL)
		*urlVar = *u
	}
	for _, test := range tests {
		in, err := parseStringTest(test.in)
		if err != nil {
			t.Fatal(err)
		}
		expected := []File{}
		for _, expStr := range test.exp {
			exp, err := parseStringTest(expStr)
			if err != nil {
				t.Fatal(err)
			}
			expected = append(expected, exp)
		}
		testIndexFn(t, ifn, in, expected)
	}

}

func parseStringTest(line string) (File, error) {
	parts := strings.Split(line, "|")

	u, err := url.Parse(parts[len(parts)-3])
	if err != nil {
		return File{}, err
	}

	secs, err := strconv.Atoi(parts[len(parts)-1])
	mtime := time.Unix(int64(secs), 0)

	return File{Path: parts[len(parts)-2], Url: *u, Mtime: mtime}, nil
}

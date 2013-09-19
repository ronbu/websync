package main

import (
	"net/http"
	"time"
	// "net/url"
	"strings"
	"testing"
)

func TestHttpStr(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "somename", time.Unix(42, 0), strings.NewReader(""))
	})

	testIndex(t, &httpHost, Http, h, []testcase{
		{in: "http://hello.com/hello.asp||42",
			exp: []string{"http://hello.com/hello.asp|hello.html|42"}},
		{in: "http://hello.com/huhu/hello.gif||42",
			exp: []string{"http://hello.com/huhu/hello.gif|hello.gif|42"}},
	})
}

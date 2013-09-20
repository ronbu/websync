package main

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// global http client
var HClient *http.Client

func init() {
	cj, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	HClient = &http.Client{Jar: cj}
}

func HGet(f File) (out File, r io.ReadCloser) {
	// TODO: check mimetypes and filename header to determine
	// correct filename and extension

	if len(f.Path) > 0 && f.Path[len(f.Path)-1] == '/' {
		name := filepath.Base(f.Url.Path)
		if name == "/" {
			// TODO: its probably better to fail here
			name = "Noname"
		}
	}

	if filepath.Ext(f.Path) == "" {
		ext := filepath.Ext(f.Url.Path)
		if ext == ".asp" || ext == ".php" {
			ext = ".html"
		}
		if ext == "" {
			ext = ".bin"
		}
		f.Path += ext
	}

	resp, err := HClient.Get(f.Url.String())
	if err != nil {
		out.Err = err
		return
	}
	if resp.StatusCode == 401 {
		resp, err = basicAuth(f.Url)
		if err != nil {
			out.Err = err
			return
		}
	}
	sc := resp.StatusCode
	if !(sc >= 200 && sc < 300) {
		return File{Err: errors.New(f.Url.String() + ": " + resp.Status)}, nil
	}
	if f.Mtime == (time.Time{}) {
		mtime, err := time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
		if err != nil {
			f.Mtime = time.Now()
		}
		f.Mtime = mtime
	}
	return f, resp.Body
}

func basicAuth(u url.URL) (resp *http.Response, err error) {
	user, password, err := Keychain(u)
	if err != nil {
		return
	}
	req, err := http.NewRequest("GET", u.String(), strings.NewReader(""))
	if err != nil {
		return
	}
	req.SetBasicAuth(user, password)
	return HClient.Do(req)
}

func grabHttp(rawurl string) (string, error) {
	u, e := url.Parse(rawurl)
	if e != nil {
		return "", e
	}
	f, r := HGet(File{Url: *u})
	if f.Err != nil {
		return "", f.Err
	}
	cont, err := ioutil.ReadAll(r)
	return string(cont), err
}

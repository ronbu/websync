package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path/filepath"
	"strings"
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

// used for testing
var httpHost url.URL

func Http(f File, files chan File, errs chan error) {
	f, err := httpGet(f)
	files <- f
	errs <- err
}

func httpGet(f File) (file File, err error) {
	if httpHost.Host != "" {
		// dependency inject test server
		f.Url.Scheme = httpHost.Scheme
		f.Url.Host = httpHost.Host
	}

	// TODO: check mimetypes and filename header to determine
	// correct filename and extension
	ext := filepath.Ext(f.Url.Path)
	if ext == ".asp" || ext == ".php" {
		ext = ".html"
	}
	if ext == "" {
		ext = ".bin"
	}

	name := filepath.Base(f.Url.Path)
	if name == "/" {
		// TODO: its probably better to fail here
		name = "Noname"
	}
	resp, err := HClient.Get(f.Url.String())
	if err != nil {
		return
	}
	if resp.StatusCode == 401 {
		resp, err = basicAuth(f.Url)
		if err != nil {
			return
		}
	}
	sc := resp.StatusCode
	if !(sc >= 200 && sc < 300) {
		return File{}, errors.New(f.Url.String() + ": " + resp.Status)
	}
	return NewFile(f, name+ext, &f.Url, nil, func() (io.ReadCloser, error) {
		return resp.Body, nil
	}), nil
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
	f, err := httpGet(File{Url: *u})
	if err != nil {
		return "", err
	}
	cont, err := f.ReadAll()
	return string(cont), err
}

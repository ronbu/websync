package main

import (
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os/exec"
	"strings"
)

var HClient *http.Client

func init() {
	cj, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	HClient = &http.Client{Jar: cj}
}

type IndexFn func(f File, files chan File, errs chan error)
type LookupFn func(f File) (indexFn IndexFn, err error)

func Lookup(f File) (indexFn IndexFn, err error) {
	const (
		HOST = iota
		PROTOCOL
		NAME
	)
	type item struct {
		name string
		kind int
		f    IndexFn
	}
	items := []item{}
	items = append(items, item{"elearning.hslu.ch", HOST, adapt(Ilias)})
	items = append(items, item{"tumblr.com", HOST, Tumblr})
	items = append(items, item{"dav", PROTOCOL, adapt(Dav)})
	items = append(items, item{"davs", PROTOCOL, adapt(Dav)})

	c := exec.Command("/usr/bin/env", "youtube-dl", "--extractor-descriptions")
	output, err := c.CombinedOutput()
	if err != nil {
		err = errors.New(err.Error() + " (trying without youtube-dl support)")
	}
	for _, line := range strings.Split(string(output), "\n") {
		items = append(items, item{strings.ToLower(line), NAME, adapt(YoutubeDl)})
	}
	for _, item := range items {
		indexFn = item.f
		switch item.kind {
		case HOST:
			if strings.HasSuffix(f.Url.Host, item.name) {
				return
			}
		case PROTOCOL:
			if f.Url.Scheme == item.name {
				return
			}
		case NAME:
			if strings.Contains(f.Url.Host, item.name) {
				return
			}
		}
	}
	return nil, err
}

type legacyFn func(u url.URL, c *http.Client, user, pw string) ([]File, error)

func adapt(l legacyFn) IndexFn {
	return func(f File, files chan File, errs chan error) {
		user, pw, err := Keychain(*f.Url)
		errs <- err
		fs, err := l(*f.Url, HClient, user, pw)
		if err != nil {
			errs <- err
		} else {
			for _, f := range fs {
				files <- f
			}
		}
	}
}

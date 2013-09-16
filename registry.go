package main

import (
	"errors"
	"net/http"
	"net/http/cookiejar"
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
	items = append(items, item{"zdf.de", HOST, Zdf})
	items = append(items, item{"tumblr.com", HOST, Tumblr})
	items = append(items, item{"dav", PROTOCOL, Dav})
	items = append(items, item{"davs", PROTOCOL, Dav})

	c := exec.Command("/usr/bin/env", "youtube-dl", "--extractor-descriptions")
	output, err := c.CombinedOutput()

	if err != nil {
		err = errors.New(err.Error() + " (trying without youtube-dl support)")
	}
	for _, line := range strings.Split(string(output), "\n") {
		items = append(items, item{strings.ToLower(line), NAME, YoutubeDl})
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

func grabHttp(u string) (string, error) {
	f := File{}
	f.FromUrl(u)
	d, e := f.ReadAll()
	return string(d), e
}

package main

import (
	"net/http"
	"net/url"
	"os/exec"
	"strings"
)

type Remote func(File, *http.Client, string, string, chan File, chan error)
type legacyRemote func(u url.URL, c *http.Client, user, pw string) ([]File, error)

func asyncAdapter(f legacyRemote) Remote {
	return func(fl File, c *http.Client, user, pw string,
		files chan File, errs chan error) {

		fs, err := f(*fl.Url, c, user, pw)
		if err != nil {
			errs <- err
		} else {
			for _, f := range fs {
				files <- f
			}
		}
		return
	}
}

type registryFn func(f File) func(chan File, chan error)

func registry(hc *http.Client) (fun registryFn, err error) {

	const (
		HOST = iota
		PROTOCOL
		NAME
	)
	type item struct {
		name string
		kind int
		f    Remote
	}
	items := []item{}
	items = append(items, item{"elearning.hslu.ch", HOST, asyncAdapter(Ilias)})
	items = append(items, item{"tumblr.com", HOST, Tumblr})
	items = append(items, item{"dav", PROTOCOL, asyncAdapter(Dav)})
	items = append(items, item{"davs", PROTOCOL, asyncAdapter(Dav)})

	c := exec.Command("/usr/bin/env", "youtube-dl", "--extractor-descriptions")
	output, err := c.CombinedOutput()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(output), "\n") {
		items = append(items, item{strings.ToLower(line), NAME, asyncAdapter(YoutubeDl)})
	}

	return func(f File) func(chan File, chan error) {
		for _, item := range items {
			match := false
			switch item.kind {
			case HOST:
				if strings.HasSuffix(f.Url.Host, item.name) {
					match = true
				}
			case PROTOCOL:
				if f.Url.Scheme == item.name {
					match = true
				}
			case NAME:
				if strings.Contains(f.Url.Host, item.name) {
					match = true
				}
			}
			if match {
				return func(files chan File, errs chan error) {
					var user, password string
					var err error
					// Make oauth usable for any Handler
					if strings.HasSuffix(f.Url.Host, "tumblr.com") {
						token, _ := handleOauth()
						user, password = token.Token, token.Secret
					} else {
						nameUri, _ := url.Parse("http://" + item.name)
						user, password, err = keychainAuth(*nameUri)
					}
					if err != nil {
						errs <- err
					}
					item.f(f, hc, user, password, files, errs)
				}
			}
		}
		return nil
	}, nil
}

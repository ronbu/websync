package main

import (
	"net/http"
	"net/url"
	"os/exec"
	"strings"
)

type Handler interface {
	Url(u *url.URL) (r *url.URL, exact bool, err error)
	Files(f File, files chan File, errs chan error)
}

type AuthFn func(*url.URL) (user, pw string, err error)
type OAuthFn func(*url.URL) (token, secret string, err error)

type NewHandlerFn func(*http.Client, *Auth) Handler

type DefaultHandler struct {
	*http.Client
	*Auth
}

type legacy func(u url.URL, c *http.Client, user, pw string) ([]File, error)
type adapter struct {
	DefaultHandler
	f legacy
}

func (a *adapter) Url(u *url.URL) (r *url.URL, exact bool, err error) {
	return u, false, nil
}
func (a *adapter) Files(f File, files chan File, errs chan error) {
	user, pw, err := a.Keychain(f.Url)
	errs <- err
	fs, err := a.f(*f.Url, a.Client, user, pw)
	if err != nil {
		errs <- err
	} else {
		for _, f := range fs {
			files <- f
		}
	}
	return
}
func Adapt(f legacy) NewHandlerFn {
	return func(c *http.Client, a *Auth) Handler {
		return &adapter{DefaultHandler{c, a}, f}
	}
}

type RegistryFn func(f File) Handler

func Registry(hc *http.Client, a *Auth) (fun RegistryFn, err error) {

	const (
		HOST = iota
		PROTOCOL
		NAME
	)
	type item struct {
		name string
		kind int
		f    NewHandlerFn
	}
	items := []item{}
	items = append(items, item{"elearning.hslu.ch", HOST, Adapt(Ilias)})
	items = append(items, item{"tumblr.com", HOST, NewTumblr})
	items = append(items, item{"dav", PROTOCOL, Adapt(Dav)})
	items = append(items, item{"davs", PROTOCOL, Adapt(Dav)})

	c := exec.Command("/usr/bin/env", "youtube-dl", "--extractor-descriptions")
	output, err := c.CombinedOutput()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(output), "\n") {
		items = append(items, item{strings.ToLower(line), NAME, Adapt(YoutubeDl)})
	}

	return func(f File) Handler {
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
				return item.f(hc, a)
			}
		}
		return nil
	}, nil
}

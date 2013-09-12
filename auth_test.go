package main

import (
	"net/url"
)

type fakeAuth struct {
	u, s string
}

func (f *fakeAuth) Keychain(u *url.URL) (user, pw string, err error) {
	return f.u, f.s, nil
}
func (f *fakeAuth) OAuth(u *url.URL) (token, secret string, err error) {
	return f.u, f.s, nil
}

package main

import (
	"github.com/mrjones/oauth"
	"net/url"
)

func init() {
	// replace auth functions for testing
	Keychain = func(_ url.URL) (u, s string, e error) { return "u", "s", nil }
	OAuth = func() (*oauth.AccessToken, error) {
		t := &oauth.AccessToken{"t", "s"}
		return t, nil
	}
}

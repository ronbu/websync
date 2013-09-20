package main

import (
	"errors"
	"net/url"
	"os/exec"
	"strings"
)

type IndexFn func(f File, ch chan File)

func Index(f File, ch chan File) {
	var fn IndexFn
	u := f.Url
	switch {
	case u.Scheme == "dav", u.Scheme == "davs":
		fn = Dav
	case strings.HasSuffix(u.Host, "zdf.de"):
		fn = Zdf
	case strings.HasSuffix(u.Host, "tumblr.com"):
		fn = Tumblr
	default:
		c := exec.Command("/usr/bin/env", "youtube-dl", "--extractor-descriptions")
		output, err := c.CombinedOutput()
		if err != nil {
			err = errors.New(err.Error() + " (trying without youtube-dl support)")
		}
		for _, line := range strings.Split(string(output), "\n") {
			if strings.Contains(u.Host, strings.ToLower(line)) {
				fn = YoutubeDl
			}
		}
	}
	if fn == nil {
		err := errors.New("No Indexer found for: " + f.Url.String())
		f.SendErr(ch, &err)
	} else {
		fn(f, ch)
	}
}

func replaceHost(u *url.URL, h string) url.URL {
	u.Host = h
	return *u
}

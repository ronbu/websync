package main

import (
	"errors"
	"os/exec"
	"strings"
)

type IndexFn func(f File, files chan File, errs chan error)
type LookupFn func(f File) (indexFn IndexFn, err error)

func Lookup(f File) (indexFn IndexFn, err error) {
	u := f.Url
	switch {
	case u.Scheme == "dav", u.Scheme == "davs":
		return Dav, err
	case strings.HasSuffix(u.Host, "zdf.de"):
		return Zdf, err
	case strings.HasSuffix(u.Host, "tumblr.de"):
		return Tumblr, err
	default:
		c := exec.Command("/usr/bin/env", "youtube-dl", "--extractor-descriptions")
		output, err := c.CombinedOutput()
		if err != nil {
			err = errors.New(err.Error() + " (trying without youtube-dl support)")
		}
		for _, line := range strings.Split(string(output), "\n") {
			if strings.Contains(u.Host, strings.ToLower(line)) {
				return YoutubeDl, err
			}
		}
	}
	return Http, err
}

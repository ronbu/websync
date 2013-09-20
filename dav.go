package main

//req = requests.Request(
//"https://elearning.hslf.Url.ch/ilias/webdav.php/hslu/ref_1669268/",method="PROPFIND",
//auth=('user','pw'))

import (
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

func Dav(f File, ch chan File) {
	if f.Url.Scheme == "davs" {
		f.Url.Scheme = "https"
	} else if f.Url.Scheme == "dav" {
		f.Url.Scheme = "http"
	}
	req, err := http.NewRequest("PROPFIND", f.Url.String(), nil)
	if err != nil {
		f.SendErr(ch, &err)
		return
	}
	user, pw, err := Keychain(f.Url)
	if err != nil {
		f.SendErr(ch, &err)
		return
	}
	req.SetBasicAuth(user, pw)
	resp, err := HClient.Do(req)
	if err != nil {
		f.SendErr(ch, &err)
		return
	}
	defer resp.Body.Close()
	msg, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		f.SendErr(ch, &err)
		return
	}
	var result Multistatus
	err = xml.Unmarshal(msg, &result)
	if err != nil {
		f.SendErr(ch, &err)
		return
	}
	for _, resp := range result.Responses {
		fileurl, err := url.Parse(f.Url.String() + resp.Name[len(f.Url.Path):])
		if err != nil {
			f.SendErr(ch, &err)
			continue
		}
		mtime, err := time.Parse(time.RFC1123, resp.Mtime)
		if err != nil {
			f.SendErr(ch, &err)
			continue
		}
		filepath := resp.Name[len(f.Url.Path):]
		filepath, err = url.QueryUnescape(filepath)
		if err != nil {
			f.SendErr(ch, &err)
			continue
		}
		if resp.Mimetype == "httpd/unix-directory" {
			continue
		}
		ch <- File{Path: filepath, Url: *fileurl, Mtime: mtime}
	}
	return
}

type Multistatus struct {
	Responses []Response `xml:"response"`
}

type Response struct {
	Name     string `xml:"href"`
	Mtime    string `xml:"propstat>prop>getlastmodified"`
	Mimetype string `xml:"propstat>prop>getcontenttype"`
}

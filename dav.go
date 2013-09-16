package main

//req = requests.Request(
//"https://elearning.hslf.Url.ch/ilias/webdav.php/hslu/ref_1669268/",method="PROPFIND",
//auth=('user','pw'))

import (
	"encoding/xml"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

func Dav(f File, files chan File, errs chan error) {
	if f.Url.Scheme == "davs" {
		f.Url.Scheme = "https"
	} else if f.Url.Scheme == "dav" {
		f.Url.Scheme = "http"
	}
	req, err := http.NewRequest("PROPFIND", f.Url.String(), nil)
	if err != nil {
		errs <- err
		return
	}
	user, pw, err := Keychain(f.Url)
	if err != nil {
		errs <- err
		return
	}
	req.SetBasicAuth(user, pw)
	resp, err := HClient.Do(req)
	if err != nil {
		errs <- err
		return
	}
	defer resp.Body.Close()
	msg, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errs <- err
		return
	}
	var result Multistatus
	err = xml.Unmarshal(msg, &result)
	if err != nil {
		errs <- err
		return
	}
	for _, resp := range result.Responses {
		fileurl, err := url.Parse(f.Url.String() + resp.Name[len(f.Url.Path):])
		if err != nil {
			errs <- err
			log.Fatalln(err)
			continue
		}
		mtime, err := time.Parse(time.RFC1123, resp.Mtime)
		if err != nil {
			errs <- err
			log.Fatalln(err)
			continue
		}
		filepath := resp.Name[len(f.Url.Path):]
		filepath, err = url.QueryUnescape(filepath)
		if err != nil {
			errs <- err
			log.Fatalln(err)
			continue
		}
		//var isDir bool
		if resp.Mimetype == "httpd/unix-directory" {
			//isDir = true
			continue
		}
		f := NewFile(f, filepath, nil, &mtime, nil)
		req, err := http.NewRequest("GET", fileurl.String(), nil)
		if err != nil {
			errs <- err
			return
		}
		req.SetBasicAuth(user, pw)
		f.FromRequest(req)
		files <- f
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

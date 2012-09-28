package main

//req = requests.Request(
//"https://elearning.hslu.ch/ilias/webdav.php/hslu/ref_1669268/",method="PROPFIND",
//auth=('user','pw'))

import (
	"encoding/xml"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

func Dav(u url.URL, c *http.Client, user, pw string) (
	files []File, err error) {
	if u.Scheme == "davs" {
		u.Scheme = "https"
	} else if u.Scheme == "dav" {
		u.Scheme = "http"
	}
	req, err := http.NewRequest("PROPFIND", u.String(), nil)
	if err != nil {
		return
	}
	req.SetBasicAuth(user, pw)
	resp, err := c.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	msg, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var result Multistatus
	err = xml.Unmarshal(msg, &result)
	if err != nil {
		return
	}
	for _, resp := range result.Responses {
		fileurl, err := url.Parse(u.String() + resp.Name[len(u.Path):])
		if err != nil {
			log.Fatalln(err)
			continue
		}
		mtime, err := time.Parse(time.RFC1123, resp.Mtime)
		if err != nil {
			log.Fatalln(err)
			continue
		}
		filepath := resp.Name[len(u.Path):]
		filepath, err = url.QueryUnescape(filepath)
		if err != nil {
			log.Fatalln(err)
			continue
		}
		//var isDir bool
		if resp.Mimetype == "httpd/unix-directory" {
			//isDir = true
			continue
		}
		files = append(files, File{
			Path:  filepath,
			Mtime: mtime,
			FileFunc: func() (r io.ReadCloser, err error) {
				req, err := http.NewRequest("GET", fileurl.String(), nil)
				if err != nil {
					return
				}
				req.SetBasicAuth(user, pw)
				res, err := c.Do(req)
				if err != nil {
					return
				}
				return res.Body, nil
			},
		})
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

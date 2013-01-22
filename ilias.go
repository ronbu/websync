package main

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	//"strings"
	"path"
)

func Ilias(uri url.URL, c *http.Client, user, pw string) (fs []File, err error) {
	modules, err := Modules(c, uri, user, pw)
	if err != nil {
		return
	}
	if len(modules) == 0 {
		err = errors.New("No Modules found")
	}
	davBase, err := DavUri(uri)
	if err != nil {
		return
	}
	for id, name := range modules {
		var uri *url.URL
		uri, err = url.Parse(davBase.String() + id)
		if err != nil {
			return
		}
		var davFs []File
		davFs, err = Dav(*uri, c, user, pw)
		if err != nil {
			return
		}
		for _, file := range davFs {
			file.Path = path.Join("/"+name, file.Path)
			fs = append(fs, file)
		}
	}
	return
}

func DavUri(u url.URL) (*url.URL, error) {
	return url.Parse(u.Scheme + "://" + u.Host + "/ilias/webdav.php/hslu/ref_")
}

func Modules(c *http.Client, uri url.URL, user, pw string) (
	modules map[string]string, err error) {
	response, err := c.PostForm(uri.String(),
		url.Values{"username": {user}, "password": {pw}})
	defer response.Body.Close()
	if response.StatusCode != 302 {
		return nil, errors.New("Login failed (wrong URL or credentials)")
	}
	if err != nil {
		return
	}
	location, err := response.Location()
	if err != nil {
		return
	}
	response, err = c.Get(location.String())
	defer response.Body.Close()
	if err != nil {
		return
	}
	html, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}
	modules = parseDesktop(string(html))
	return
}

func parseDesktop(html string) map[string]string {
	modules := make(map[string]string, 30)
	rx := regexp.MustCompile(
		`<a.*?ref_id=(\d+)&cmd=&.*?class="il_ContainerItemTitle".*?>(.*?)</a>`)
	for _, matches := range rx.FindAllStringSubmatch(html, 100000) {
		modules[matches[1]] = matches[2]
	}
	return modules
}

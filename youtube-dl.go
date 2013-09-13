package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

func YoutubeDl(u url.URL, c *http.Client, user, password string) (
	files []File, err error) {
	base, rmTmp := TempDir()
	defer rmTmp()

	cmd := exec.Command("/usr/bin/env",
		"youtube-dl", "--skip-download", "--write-info-json", u.String())
	cmd.Dir = base
	err = cmd.Run()
	if err != nil {
		return
	}

	baseDir, err := os.Open(base)
	if err != nil {
		return
	}
	infoFiles, err := baseDir.Readdirnames(-1)
	if err != nil {
		return
	}

	for _, name := range infoFiles {
		name = filepath.Join(base, name)

		var content []byte
		content, err = ioutil.ReadFile(name)
		if err != nil {
			return
		}
		var info info
		err = json.Unmarshal(content, &info)
		if err != nil {
			return
		}

		var year, month, day int
		year, err = strconv.Atoi(info.Upload_date[0:4])
		if err != nil {
			return
		}
		month, err = strconv.Atoi(info.Upload_date[4:6])
		if err != nil {
			return
		}
		day, err = strconv.Atoi(info.Upload_date[6:8])
		if err != nil {
			return
		}
		mtime := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

		files = append(files, File{
			Url:   url.URL{Path: info.Title + "." + info.Ext},
			Mtime: mtime,
			FileFunc: func() (r io.ReadCloser, err error) {
				base, rmTmp := TempDir()
				if err != nil {
					return
				}
				// Behaviour in OS X: The temporary file will be
				// automatically deleted after it is closed
				// TODO: Find out if this works the same on other OS's
				defer rmTmp()

				cmd := exec.Command("/usr/bin/env",
					"youtube-dl", "--id", u.String())
				cmd.Dir = base
				// cmd.Stdout = os.Stderr
				// cmd.Stderr = os.Stderr
				err = cmd.Run()
				if err != nil {
					return
				}

				return os.Open(filepath.Join(base, info.Id+"."+info.Ext))
			},
		})
	}

	return
}

type info struct {
	Upload_date, Title, Id, Ext string
}

// type info struct{
//   Upload_date, Playlist,	Description ,Format string,
//   Url ,	Title ,Id ,	Thumbnail Ext ,	Stitle string,
//   Extractor ,Uploader ,Duration ,Fulltitle string,
//   Player_url ,	Uploader_id ,	Subtitles string,
//   Playlist_index int,
// }

func TempDir() (name string, stop func()) {
	name, err := ioutil.TempDir("", "websync")
	check(err)
	name, err = filepath.EvalSymlinks(name)
	check(err)
	return name, func() {
		check(os.RemoveAll(name))
	}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

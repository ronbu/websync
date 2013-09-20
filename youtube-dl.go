package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func YoutubeDl(f File, ch chan File) {
	base, rmTmp := TempDir()
	defer rmTmp()

	cmd := exec.Command("/usr/bin/env",
		"youtube-dl", "--skip-download", "--write-info-json", f.Url.String())
	cmd.Dir = base
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		err = errors.New("youtube-dl does not support: " + f.Url.String())
		f.SendErr(ch, &err)
		return
	}

	baseDir, err := os.Open(base)
	if err != nil {
		f.SendErr(ch, &err)
		return
	}
	infoFiles, err := baseDir.Readdirnames(-1)
	if err != nil {
		f.SendErr(ch, &err)
		return
	}

	for _, name := range infoFiles {
		name = filepath.Join(base, name)

		var content []byte
		content, err = ioutil.ReadFile(name)
		if err != nil {
			f.SendErr(ch, &err)
			continue
		}
		var info info
		err = json.Unmarshal(content, &info)
		if err != nil {
			f.SendErr(ch, &err)
			continue
		}

		f.Path += strings.Replace(info.Title, "/", "_", -1) + "." + info.Ext

		url, err := url.Parse(info.Url)
		if err != nil {
			f.SendErr(ch, &err)
			continue
		}
		f.Url = *url

		upd := info.Upload_date
		mtime := time.Time{}
		if len(upd) != 8 {
			err = errors.New("YoutubeDl: Invalid upload date: " + info.Upload_date)
			f.SendErr(ch, &err)
		} else {
			year, e1 := strconv.Atoi(upd[0:4])
			month, e2 := strconv.Atoi(upd[4:6])
			day, e3 := strconv.Atoi(upd[6:8])
			if e1 != nil || e2 != nil || e3 != nil {
				err = errors.New("YoutubeDl: Could not parse Upload date")
				f.SendErr(ch, &err)
			} else {
				mtime = time.Date(
					year,
					time.Month(month),
					day, 0, 0, 0, 0,
					time.UTC)
			}
		}
		f.Mtime = mtime

		ch <- f.SetLeaf()
	}

	return
}

type info struct {
	Upload_date, Title, Id, Ext, Url string
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

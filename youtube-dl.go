package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func YoutubeDl(f File, fs chan File, es chan error) {
	base, rmTmp := TempDir()
	defer rmTmp()

	cmd := exec.Command("/usr/bin/env",
		"youtube-dl", "--skip-download", "--write-info-json", f.Url.String())
	cmd.Dir = base
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		es <- errors.New("youtube-dl does not support: " + f.Url.String())
		return
	}

	baseDir, err := os.Open(base)
	if err != nil {
		es <- err
		return
	}
	infoFiles, err := baseDir.Readdirnames(-1)
	if err != nil {
		es <- err
		return
	}

	for _, name := range infoFiles {
		name = filepath.Join(base, name)

		var content []byte
		content, err = ioutil.ReadFile(name)
		if err != nil {
			es <- err
			return
		}
		var info info
		err = json.Unmarshal(content, &info)
		if err != nil {
			es <- err
			return
		}

		upd := info.Upload_date
		mtime := unixZerotime
		if len(upd) != 8 {
			es <- errors.New("YoutubeDl: Invalid upload date: " + info.Upload_date)
		} else {
			year, e1 := strconv.Atoi(upd[0:4])
			month, e2 := strconv.Atoi(upd[4:6])
			day, e3 := strconv.Atoi(upd[6:8])
			if e1 != nil || e2 != nil || e3 != nil {
				es <- errors.New("YoutubeDl: Could not parse Upload date")
			} else {
				mtime = time.Date(
					year,
					time.Month(month),
					day, 0, 0, 0, 0,
					time.UTC)
			}
		}

		f.Append(strings.Replace(info.Title, "/", "_", -1) + "." + info.Ext)
		f.Mtime = mtime
		f.Read = func() (r io.ReadCloser, err error) {
			base, rmTmp := TempDir()
			if err != nil {
				return
			}
			// Behaviour in OS X: The temporary file will be
			// automatically deleted after it is closed
			// TODO: Find out if this works the same on other OS's
			defer rmTmp()

			cmd := exec.Command("/usr/bin/env",
				"youtube-dl", "--id", f.Url.String())
			cmd.Dir = base
			// cmd.Stdout = os.Stderr
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			if err != nil {
				return nil, errors.New(
					fmt.Sprintf(
						"youtube-dl failed: %v (%v)",
						f.Url.String(),
						err.Error()))
			}

			return os.Open(filepath.Join(base, info.Id+"."+info.Ext))
		}

		fs <- f
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

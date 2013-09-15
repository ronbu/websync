package main

import (
	"net/url"
	"os"
	"testing"
	"time"
)

func TestLocalNew(t *testing.T) {
	testLocal(t, func(f File) File {
		return f
	}, true)
}

func TestLocalOverwriteOlder(t *testing.T) {
	testLocal(t, func(f File) File {
		err := Local(f)
		if err != nil {
			t.Error(err)
		}
		f.Mtime = f.Mtime.Add(time.Second)
		return f
	}, true)
}

func TestLocalNotOverwriteNewer(t *testing.T) {
	testLocal(t, func(f File) File {
		err := Local(f)
		if err != nil {
			t.Error(err)
		}
		f.Mtime = f.Mtime.Add(-time.Second)
		return f
	}, false)
}

func TestCreateDirs(t *testing.T) {
	testLocal(t, func(f File) File {
		f.Url.Path += "/a/dir/oh/uh/hi/ho"
		return f
	}, true)
}

// Trying to overwrite a directory fails
func TestLocalOverwriteDir(t *testing.T) {
	tmp, rm := TempDir()
	defer rm()
	f := someTestFile(tmp)
	os.Mkdir(f.Url.Path, 777)
	os.Chtimes(f.Url.Path, time.Now(), time.Now().Add(-time.Hour))
	if Local(f) == nil {
		t.Error("should have failed")
	}
}

func testLocal(t *testing.T, init func(File) File, overwrite bool) {
	tmp, rm := TempDir()
	of := someTestFile(tmp)
	f := init(of)
	err := Local(f)
	if err != nil {
		t.Error(err)
	}
	if overwrite {
		of = f
	}
	checkFile(t, of)
	rm()
}

func someTestFile(tmp string) File {
	return File{
		Url:      url.URL{Path: tmp + "/a"},
		Mtime:    time.Now(),
		FileFunc: stringReadFn("test"),
	}
}

func createSomeFile(tmp string) {
	return
}

func checkFile(t *testing.T, f File) {
	st, err := os.Stat(f.Url.Path)

	f.Mtime = removeSubSecond(f.Mtime)

	if err != nil && os.IsNotExist(err) {
		t.Error("File does not exist:", f.Url)
	} else {
		if !(st.ModTime().Equal(f.Mtime)) {
			t.Errorf("Not overwritten")
		}
	}
}

// OSX does not store time resolutions below seconds
func removeSubSecond(in time.Time) time.Time {
	return time.Date(
		in.Year(),
		in.Month(),
		in.Day(),
		in.Hour(),
		in.Minute(),
		in.Second(),
		0, in.Location())
}

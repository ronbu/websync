package main

import (
	"os"
	"strconv"
	"testing"
	"time"
)

func TestSync(t *testing.T) {
	// Sync is depth first
	input := [][]int{{1, 0, 6}, {0, 4, 5}, {2, 3}}
	expN := 1
	fs, es := injectableSync("", "", fakeLookup(input),
		func(f File) error {
			n, _ := strconv.Atoi(f.Path())
			if n != expN {
				t.Errorf("%d should have been %d", n, expN)
			}
			expN++
			return nil
		})
Loop:
	for {
		select {
		case _, ok := <-fs:
			if !ok {
				break Loop
			}
		case <-es:
		}
	}
	expected := flattenInts(input)
	if expN < len(expected) {
		t.Error("Missing Files:", expected[expN:])
	}
}

func fakeLookup(nums [][]int) LookupFn {
	i := 0
	return func(f File) (IndexFn, error) {
		return func(f File, files chan File, errs chan error) {
			for _, n := range nums[i] {
				nf := f
				nf.FromString("")
				if n == 0 {
					i++
					nf.Read = nil
				}
				files <- nf.Append(strconv.Itoa(n))
			}
		}, nil
	}
}

func flattenInts(ints [][]int) []int {
	res := []int{}
	for _, g := range ints {
		for _, n := range g {
			if n == 0 {
				continue
			}
			res = append(res, n)
		}
	}
	return res
}

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

func TestLocalCreateDirs(t *testing.T) {
	testLocal(t, func(f File) File {
		return f.Append("a/dir/oh/uh/hi/ho")
	}, true)
}

// Trying to overwrite a directory fails
func TestLocalOverwriteDir(t *testing.T) {
	tmp, rm := TempDir()
	defer rm()
	f := someTestFile(tmp)
	err := os.Mkdir(f.Path(), 777)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chtimes(f.Path(), time.Now(), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
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
	f := File{Mtime: time.Now(), path: tmp + "/a"}
	f.FromString("test")
	return f
}

func checkFile(t *testing.T, f File) {
	st, err := os.Stat(f.Path())

	f.Mtime = removeSubSecond(f.Mtime)

	if err != nil && os.IsNotExist(err) {
		t.Error("File does not exist:", f.Path())
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

// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package interfacer

import (
	"go/build"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var skipDir = regexp.MustCompile(`^(testdata|vendor|_.*|\..+)$`)

func getDirsGopath(gopath, d string) ([]string, error) {
	local := d == "." || strings.HasPrefix(d, "./")
	var dirs []string
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if skipDir.MatchString(info.Name()) {
			return filepath.SkipDir
		}
		if local {
			if !strings.HasPrefix(path, "./") {
				path = "./" + path
			}
		} else {
			path = path[len(gopath)+5:]
		}
		dirs = append(dirs, path)
		return nil
	}
	if !local {
		d = filepath.Join(gopath, "src", d)
	}
	if err := filepath.Walk(d, walkFn); err != nil {
		return nil, err
	}
	return dirs, nil
}

func getDirs(d string) ([]string, error) {
	var err error
	for _, gopath := range filepath.SplitList(build.Default.GOPATH) {
		var dirs []string
		if dirs, err = getDirsGopath(gopath, d); err == nil {
			return dirs, nil
		}
	}
	return nil, err
}

func recurse(args []string) ([]string, error) {
	if len(args) == 0 {
		return []string{"."}, nil
	}
	if strings.HasSuffix(args[0], ".go") {
		return args, nil
	}
	var paths []string
	for _, p := range args {
		if !strings.HasSuffix(p, "/...") {
			paths = append(paths, p)
			continue
		}
		d := p[:len(p)-4]
		dirs, err := getDirs(d)
		if err != nil {
			return nil, err
		}
		paths = append(paths, dirs...)
	}
	return paths, nil
}

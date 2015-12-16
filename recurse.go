// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var skipDir = regexp.MustCompile(`^(testdata|vendor|_.*|\.\+)$`)

func getDirs(d string) ([]string, error) {
	var dirs []string
	if !strings.HasPrefix(d, "./") {
		return nil, fmt.Errorf("TODO: recursing into non-local import paths")
	}
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if skipDir.MatchString(info.Name()) {
			return filepath.SkipDir
		}
		if info.IsDir() {
			if !strings.HasPrefix(path, "./") {
				path = "./" + path
			}
			dirs = append(dirs, path)
		}
		return nil
	}
	if err := filepath.Walk(d, walkFn); err != nil {
		return nil, err
	}
	return dirs, nil
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

/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func doTest(t *testing.T, inPath string) {
	var b bytes.Buffer
	if err := checkPaths([]string{inPath}, &b); err != nil {
		t.Fatal(err)
	}
	outPath := inPath + ".out"
	expBytes, err := ioutil.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	exp := string(expBytes)
	got := b.String()
	if exp != got {
		t.Fatalf("Mismatch in %s.\nExpected:\n%sGot:\n%s",
			outPath, exp, got)
	}
}

func TestFile(t *testing.T) {
	inGlob := filepath.Join("testdata", "*.go")
	matches, err := filepath.Glob(inGlob)
	if err != nil {
		t.Fatal(err)
	}
	for _, inPath := range matches {
		doTest(t, inPath)
	}
}

func isDir(p string) (bool, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return false, err
	}
	return fi.Mode().IsDir(), nil
}

func TestDir(t *testing.T) {
	dirGlob := filepath.Join("testdata", "*")
	matches, err := filepath.Glob(dirGlob)
	if err != nil {
		t.Fatal(err)
	}
	for _, inPath := range matches {
		dir, err := isDir(inPath)
		if err != nil {
			t.Fatal(err)
		}
		if !dir {
			continue
		}
		doTest(t, inPath)
	}
}

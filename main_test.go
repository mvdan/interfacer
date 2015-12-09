/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func doTest(t *testing.T, inPaths []string, outPath string) {
	var b bytes.Buffer
	if err := checkPaths(inPaths, &b); err != nil {
		t.Fatal(err)
	}
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

func TestSingleFile(t *testing.T) {
	testsGlob := filepath.Join("testdata", "*.go")
	matches, err := filepath.Glob(testsGlob)
	if err != nil {
		t.Fatal(err)
	}
	for _, inPath := range matches {
		outPath := inPath + ".out"
		doTest(t, []string{inPath}, outPath)
	}
}

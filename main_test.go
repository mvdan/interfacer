/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func doTest(t *testing.T, p string) {
	if strings.HasSuffix(p, ".out") {
		return
	}
	inPath := p
	if !strings.HasSuffix(p, ".go") {
		inPath += "/..."
	}
	var b bytes.Buffer
	if err := checkPaths([]string{inPath}, &b); err != nil {
		t.Fatal(err)
	}
	outPath := p + ".out"
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

func TestAll(t *testing.T) {
	if err := os.Chdir("testdata"); err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob("*")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range matches {
		doTest(t, p)
	}
	if err := os.Chdir(".."); err != nil {
		t.Fatal(err)
	}
}

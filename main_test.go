/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"bytes"
	"go/token"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func doTest(t *testing.T, inPaths []string, outPath string) {
	p := &goPkg{
		fset: token.NewFileSet(),
	}
	for _, inPath := range inPaths {
		if err := p.parsePath(inPath); err != nil {
			t.Fatal(err)
		}
	}
	var b bytes.Buffer
	if err := p.check(&b); err != nil {
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

func TestSingle(t *testing.T) {
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

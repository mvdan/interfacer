/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"bytes"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestAll(t *testing.T) {
	testsGlob := filepath.Join("testdata", "*.go")
	matches, err := filepath.Glob(testsGlob)
	if err != nil {
		t.Fatal(err)
	}
	for _, inPath := range matches {
		f, err := os.Open(inPath)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		outPath := inPath + ".out"
		expBytes, err := ioutil.ReadFile(outPath)
		if err != nil {
			t.Fatal(err)
		}
		p := &goPkg{
			fset: token.NewFileSet(),
		}
		if err := p.parsePath(inPath); err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if err := p.check(&b); err != nil {
			t.Fatal(err)
		}
		exp := string(expBytes)
		got := b.String()
		if exp != got {
			t.Fatalf("Mismatch in %s.\nExpected:\n%sGot:\n%s",
				inPath, exp, got)
		}
	}
}

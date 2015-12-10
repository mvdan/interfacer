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

func want(t *testing.T, p string) (string, bool) {
	outBytes, err := ioutil.ReadFile(p + ".out")
	if err == nil {
		return string(outBytes), false
	}
	if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	errBytes, err := ioutil.ReadFile(p + ".err")
	if err == nil {
		return string(errBytes), true
	}
	if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return "", false
}

func doTest(t *testing.T, p string) {
	if strings.HasSuffix(p, ".out") {
		return
	}
	inPath := p
	if !strings.HasSuffix(p, ".go") {
		inPath += "/..."
	}
	var b bytes.Buffer
	err := checkPaths([]string{inPath}, &b)
	exp, wantErr := want(t, p)
	if wantErr {
		if err == nil {
			t.Fatalf("Wanted error in %s, but none found.", p)
		}
		got := err.Error()
		if got[len(got)-1] != '\n' {
			got += "\n"
		}
		if exp != got {
			t.Fatalf("Error mismatch in %s:\nExpected:\n%sGot:\n%s",
				p, exp, got)
		}
		return
	}
	if err != nil {
		t.Fatalf("Did not want error in %s:\n%v", p, err)
	}
	got := b.String()
	if exp != got {
		t.Fatalf("Output mismatch in %s:\nExpected:\n%sGot:\n%s",
			p, exp, got)
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

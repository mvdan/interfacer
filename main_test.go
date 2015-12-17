// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	name = flag.String("name", "", "name of the test to run")
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
	if strings.HasSuffix(p, ".out") || strings.HasSuffix(p, ".err") {
		return
	}
	inPath := p
	if !strings.HasSuffix(p, ".go") {
		inPath = "./" + inPath + "/..."
	}
	var b bytes.Buffer
	err := checkArgs([]string{inPath}, &b)
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
	*verbose = true
	if err := os.Chdir("testdata"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(".."); err != nil {
			t.Fatal(err)
		}
	}()
	var tests []string
	if *name != "" {
		tests = []string{*name}
	} else {
		var err error
		if tests, err = filepath.Glob("*"); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range tests {
		doTest(t, p)
	}
}

func BenchmarkImportStd(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if err := typesInit(); err != nil {
			b.Fatal(err)
		}
		if _, err := c.Load(); err != nil {
			b.Fatal(err)
		}
	}
}

// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package interfacer

import (
	"bytes"
	"flag"
	"go/build"
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
	if !strings.HasPrefix(p, "./") {
		p = filepath.Join("src", p)
	}
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
	inPath := p
	if !strings.HasSuffix(p, ".go") {
		inPath = inPath + "/..."
	}
	var b bytes.Buffer
	err := CheckArgs([]string{inPath}, &b, true)
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
	defer func() {
		if err := os.Chdir(".."); err != nil {
			t.Fatal(err)
		}
	}()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	build.Default.GOPATH = wd
	if *name != "" {
		doTest(t, *name)
		return
	}
	dirs, err := filepath.Glob("*")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		if strings.HasSuffix(d, ".out") || strings.HasSuffix(d, ".err") {
			continue
		}
		if d == "src" {
			continue
		}
		doTest(t, "./"+d)
	}
	dirs, err = filepath.Glob("src/*")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		if strings.HasSuffix(d, ".out") || strings.HasSuffix(d, ".err") {
			continue
		}
		doTest(t, d[4:])
	}
}

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
	if !strings.HasPrefix(p, "./") && !strings.HasSuffix(p, ".go") {
		p = filepath.Join("src", p)
	}
	if strings.HasSuffix(p, "/...") {
		p = p[:len(p)-4]
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
	t.Fatalf("Output file not found for %s", p)
	return "", false
}

func doTest(t *testing.T, p string) {
	exp, wantErr := want(t, p)
	if strings.HasPrefix(exp, "/") {
		exp = build.Default.GOPATH + exp
	}
	doTestWant(t, p, exp, wantErr, p)
}

func endNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

func mapCopy(m map[string]string) map[string]string {
	mc := make(map[string]string, len(m))
	for k, v := range m {
		mc[k] = v
	}
	return mc
}

func doTestWant(t *testing.T, name, exp string, wantErr bool, args ...string) {
	ifacesCopy := mapCopy(ifaces)
	funcsCopy := mapCopy(funcs)
	defer func() {
		ifaces = ifacesCopy
		funcs = funcsCopy
	}()
	var b bytes.Buffer
	err := CheckArgs(args, &b, true)
	exp = endNewline(exp)
	if wantErr {
		if err == nil {
			t.Fatalf("Wanted error in %s, but none found.", name)
		}
		got := endNewline(err.Error())
		if exp != got {
			t.Fatalf("Error mismatch in %s:\nExpected:\n%sGot:\n%s",
				name, exp, got)
		}
		return
	}
	if err != nil {
		t.Fatalf("Did not want error in %s:\n%v", name, err)
	}
	got := endNewline(b.String())
	if exp != got {
		t.Fatalf("Output mismatch in %s:\nExpected:\n%sGot:\n%s",
			name, exp, got)
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
	paths, err := filepath.Glob("*")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		if strings.HasSuffix(p, ".out") || strings.HasSuffix(p, ".err") {
			continue
		}
		if p == "src" {
			continue
		}
		if strings.HasSuffix(p, ".go") {
			// Go file
			doTest(t, p)
		} else {
			// local recursive
			doTest(t, "./"+p+"/...")
		}
	}
	dirs, err := filepath.Glob("src/*")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		if strings.HasSuffix(d, ".out") || strings.HasSuffix(d, ".err") {
			continue
		}
		// non-local recursive
		doTest(t, d[4:]+"/...")
		// local recursive
		doTest(t, "./"+d+"/...")
	}
	// local non-recursive
	doTest(t, "./single")
	// non-local non-recursive
	doTest(t, "single")
	// non-existent Go file
	doTest(t, "missing.go")
	// local non-existent non-recursive
	doTest(t, "./missing")
	// non-local non-existent non-recursive
	doTest(t, "missing")
	// local non-existent recursive
	doTest(t, "./missing-rec/...")
	doTestWant(t, "wrong-args", "named files must be .go files: bar", true, "foo.go", "bar")
	// make sure we don't miss a package's imports
	doTestWant(t, "grab-import", "grab-import", false, "grab-import")
}

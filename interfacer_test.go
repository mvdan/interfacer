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
	name  = flag.String("name", "", "name of the test to run")
	write = flag.Bool("write", false, "write output files")
)

func basePath(p string) string {
	if !strings.HasPrefix(p, "./") && !strings.HasSuffix(p, ".go") {
		p = filepath.Join("src", p)
	}
	if strings.HasSuffix(p, "/...") {
		p = p[:len(p)-4]
	}
	return p
}

func want(t *testing.T, p string) (string, bool) {
	base := basePath(p)
	outBytes, err := ioutil.ReadFile(base + ".out")
	if err == nil {
		return string(outBytes), false
	}
	if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	errBytes, err := ioutil.ReadFile(base + ".err")
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
	if *write {
		doTestWrite(t, p)
		return
	}
	exp, wantErr := want(t, p)
	if strings.HasPrefix(exp, "/") {
		exp = filepath.Join(build.Default.GOPATH, "local", exp)
	}
	doTestWant(t, p, exp, wantErr, p)
}

func doTestWrite(t *testing.T, p string) {
	var b bytes.Buffer
	err := CheckArgs([]string{p}, &b, true)
	var outPath, outCont, rmPath string
	base := basePath(p)
	if err != nil {
		outPath = base + ".err"
		rmPath = base + ".out"
		outCont = err.Error()
	} else {
		outPath = base + ".out"
		rmPath = base + ".err"
		outCont = b.String()
	}
	outCont = endNewline(outCont)
	if err := ioutil.WriteFile(outPath, []byte(outCont), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(rmPath); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
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
	if *write {
		return
	}
	ifacesCopy := mapCopy(ifaces)
	funcsCopy := mapCopy(funcs)
	defer func() {
		ifaces = ifacesCopy
		funcs = funcsCopy
	}()
	var b bytes.Buffer
	if len(args) == 0 {
		args = []string{name}
	}
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

func runFileTests(t *testing.T) {
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
		if !strings.HasSuffix(p, ".go") {
			continue
		}
		doTest(t, p)
	}
}

func runLocalTests(t *testing.T) {
	if err := os.Chdir("local"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(".."); err != nil {
			t.Fatal(err)
		}
	}()
	paths, err := filepath.Glob("*")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		if strings.HasSuffix(p, ".out") || strings.HasSuffix(p, ".err") {
			continue
		}
		doTest(t, "./"+p+"/...")
	}
	// non-recursive
	doTest(t, "./single")
}

func runNonlocalTests(t *testing.T) {
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
	// non-recursive
	doTest(t, "single")
	// make sure we don't miss a package's imports
	doTestWant(t, "grab-import", "grab-import", false)
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
	runFileTests(t)
	runLocalTests(t)
	runNonlocalTests(t)
	// non-existent Go file
	doTestWant(t, "missing.go", "open missing.go: no such file or directory", true)
	// local non-existent non-recursive
	doTestWant(t, "./missing", "no initial packages were loaded", true)
	// non-local non-existent non-recursive
	doTestWant(t, "missing", "no initial packages were loaded", true)
	// local non-existent recursive
	doTestWant(t, "./missing-rec/...", "lstat ./missing-rec: no such file or directory", true)
	// Mixing Go files and dirs
	doTestWant(t, "wrong-args", "named files must be .go files: bar", true, "foo.go", "bar")
}

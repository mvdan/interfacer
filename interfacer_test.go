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
	t.Fatalf("Output file not found: %s.(out|err)", base)
	return "", false
}

func doTest(t *testing.T, p string) {
	if *write {
		doTestWrite(t, p)
		return
	}
	exp, wantErr := want(t, p)
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

func doTestWant(t *testing.T, name, exp string, wantErr bool, args ...string) {
	if *write {
		return
	}
	var b bytes.Buffer
	switch len(args) {
	case 0:
		args = []string{name}
	case 1:
		if args[0] == "" {
			args = nil
		}
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

func inputPaths(t *testing.T, glob string) []string {
	all, err := filepath.Glob(glob)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, p := range all {
		if strings.HasSuffix(p, ".out") || strings.HasSuffix(p, ".err") {
			continue
		}
		paths = append(paths, p)
	}
	return paths
}

func chdirUndo(t *testing.T, d string) func() {
	if err := os.Chdir(d); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(".."); err != nil {
			t.Fatal(err)
		}
	}
}

func runFileTests(t *testing.T, paths ...string) {
	defer chdirUndo(t, "files")()
	if len(paths) == 0 {
		paths = inputPaths(t, "*")
	}
	for _, p := range paths {
		doTest(t, p)
	}
}

func runLocalTests(t *testing.T, paths ...string) {
	defer chdirUndo(t, "local")()
	if len(paths) == 0 {
		for _, p := range inputPaths(t, "*") {
			paths = append(paths, "./"+p+"/...")
		}
		// non-recursive
		paths = append(paths, "./single")
	}
	for _, p := range paths {
		doTest(t, p)
	}
	doTestWant(t, "no-args", ".", false, "")
}

func runNonlocalTests(t *testing.T) {
	defer chdirUndo(t, "src")()
	paths := inputPaths(t, "*")
	for _, p := range paths {
		doTest(t, p+"/...")
	}
	// local recursive
	doTest(t, "./nested/...")
	// non-recursive
	doTest(t, "single")
	// make sure we don't miss a package's imports
	doTestWant(t, "grab-import", "grab-import", false)
	defer chdirUndo(t, "nested/pkg")()
	// relative paths
	doTestWant(t, "rel-path", "nested/pkg\nsimple.go:12:17: rc can be nested/pkg.Closer", false, "./...")
}

func TestAll(t *testing.T) {
	defer chdirUndo(t, "testdata")()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	build.Default.GOPATH = wd
	switch {
	case *name == "":
	case strings.HasSuffix(*name, ".go"):
		runFileTests(t, *name)
		return
	case strings.HasPrefix(*name, "./"):
		runLocalTests(t, *name)
		return
	default:
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

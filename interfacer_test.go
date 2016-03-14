// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package interfacer

import (
	"bytes"
	"flag"
	"fmt"
	"go/build"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const testdata = "testdata"

var (
	name     = flag.String("name", "", "name of the test to run")
	warnsRe  = regexp.MustCompile(`^WARN (.*)\n?$`)
	singleRe = regexp.MustCompile(`([^ ]*) can be ([^ ]*)`)
)

func goFiles(p string) ([]string, error) {
	if strings.HasSuffix(p, ".go") {
		return []string{p}, nil
	}
	dirs, err := recurse(p)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, dir := range dirs {
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			paths = append(paths, filepath.Join(dir, file.Name()))
		}
	}
	return paths, nil
}

func wantedWarnings(t *testing.T, p string) []Warn {
	paths, err := goFiles(p)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var warns []Warn
	for _, path := range paths {
		src, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer src.Close()
		f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		for _, group := range f.Comments {
			m := warnsRe.FindStringSubmatch(group.Text())
			if m == nil {
				continue
			}
			for _, m := range singleRe.FindAllStringSubmatch(m[1], -1) {
				warns = append(warns, Warn{
					Pos: token.Position{
						Filename: path,
					},
					Name: m[1],
					Type: m[2],
				})
			}
		}
	}
	return warns
}

func doTest(t *testing.T, p string) {
	warns := wantedWarnings(t, p)
	doTestWarns(t, p, warns, p)
}

func warnsEqual(got, want []Warn) bool {
	if len(got) != len(want) {
		return false
	}
	for i, w1 := range got {
		w2 := want[i]
		if w1.Name != w2.Name {
			return false
		}
		if w1.Type != w2.Type {
			return false
		}
	}
	return true
}

func warnsJoin(warns []Warn) string {
	var b bytes.Buffer
	for _, warn := range warns {
		fmt.Fprintln(&b, warn.String())
	}
	return b.String()
}

func doTestWarns(t *testing.T, name string, exp []Warn, args ...string) {
	got, err := CheckArgsList(args)
	if err != nil {
		t.Fatalf("Did not want error in %s:\n%v", name, err)
	}
	if !warnsEqual(exp, got) {
		t.Fatalf("Output mismatch in %s:\nExpected:\n%sGot:\n%s",
			name, warnsJoin(exp), warnsJoin(got))
	}
}

func endNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

func doTestString(t *testing.T, name, exp string, args ...string) {
	var b bytes.Buffer
	switch len(args) {
	case 0:
		args = []string{name}
	case 1:
		if args[0] == "" {
			args = nil
		}
	}
	err := CheckArgsOutput(args, &b, true)
	if err != nil {
		t.Fatalf("Did not want error in %s:\n%v", name, err)
	}
	exp = endNewline(exp)
	got := b.String()
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
		paths = append(paths, p)
	}
	return paths
}

func chdirUndo(t *testing.T, d string) func() {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(d); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(wd); err != nil {
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
	doTestString(t, "no-args", ".", "")
}

func runNonlocalTests(t *testing.T, paths ...string) {
	defer chdirUndo(t, "src")()
	if len(paths) > 0 {
		for _, p := range paths {
			doTest(t, p)
		}
		return
	}
	paths = inputPaths(t, "*")
	for _, p := range paths {
		doTest(t, p+"/...")
	}
	// local recursive
	doTest(t, "./nested/...")
	// non-recursive
	doTest(t, "single")
	// make sure we don't miss a package's imports
	doTestString(t, "grab-import", "grab-import\ngrab-import/use.go:27:15: s can be def2.Fooer")
	defer chdirUndo(t, "nested/pkg")()
	// relative paths
	doTestString(t, "rel-path", "nested/pkg\nsimple.go:12:17: rc can be Closer", "./...")
}

func TestMain(m *testing.M) {
	flag.Parse()
	if err := os.Chdir(testdata); err != nil {
		panic(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	build.Default.GOPATH = wd
	os.Exit(m.Run())
}

func TestCheckWarnings(t *testing.T) {
	switch {
	case *name == "":
	case strings.HasSuffix(*name, ".go"):
		runFileTests(t, *name)
		return
	case strings.HasPrefix(*name, "./"):
		runLocalTests(t, *name)
		return
	default:
		runNonlocalTests(t, *name)
		return
	}
	runFileTests(t)
	runLocalTests(t)
	runNonlocalTests(t)
}

func doTestError(t *testing.T, name, exp string, args ...string) {
	switch len(args) {
	case 0:
		args = []string{name}
	case 1:
		if args[0] == "" {
			args = nil
		}
	}
	err := CheckArgsOutput(args, ioutil.Discard, false)
	if err == nil {
		t.Fatalf("Wanted error in %s, but none found.", name)
	}
	got := err.Error()
	if exp != got {
		t.Fatalf("Error mismatch in %s:\nExpected:\n%sGot:\n%s",
			name, exp, got)
	}
}

func TestErrors(t *testing.T) {
	// non-existent Go file
	doTestError(t, "missing.go", "open missing.go: no such file or directory")
	// local non-existent non-recursive
	doTestError(t, "./missing", "no initial packages were loaded")
	// non-local non-existent non-recursive
	doTestError(t, "missing", "no initial packages were loaded")
	// local non-existent recursive
	doTestError(t, "./missing-rec/...", "lstat ./missing-rec: no such file or directory")
	// Mixing Go files and dirs
	doTestError(t, "wrong-args", "named files must be .go files: bar", "foo.go", "bar")
}

func TestExtraArg(t *testing.T) {
	err := CheckArgsOutput([]string{"single", "--", "foo", "bar"}, ioutil.Discard, false)
	got := err.Error()
	want := "unwanted extra args: [foo bar]"
	if got != want {
		t.Fatalf("Error mismatch:\nExpected:\n%sGot:\n%s", want, got)
	}
}

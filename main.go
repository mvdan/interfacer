/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
)

var (
	verbose = flag.Bool("v", false, "print the names of packages as they are checked")
)

func init() {
	if err := typesInit(); err != nil {
		errExit(err)
	}
}

func main() {
	flag.Parse()
	if err := checkPaths(flag.Args(), os.Stdout); err != nil {
		errExit(err)
	}
}

func errExit(err error) {
	fmt.Fprintf(os.Stderr, "%v\n", err)
	os.Exit(1)
}

type call struct {
	params  []types.Type
	results []types.Type
}

var toToken = map[string]token.Token{
	"byte":    token.INT,
	"int":     token.INT,
	"int8":    token.INT,
	"int16":   token.INT,
	"int32":   token.INT,
	"int64":   token.INT,
	"uint":    token.INT,
	"uint8":   token.INT,
	"uint16":  token.INT,
	"uint32":  token.INT,
	"uint64":  token.INT,
	"string":  token.STRING,
	"float32": token.FLOAT,
	"float64": token.FLOAT,
}

func paramsMatch(wanted, got []types.Type) bool {
	if len(wanted) != len(got) {
		return false
	}
	for i, t1 := range wanted {
		t2 := got[i]
		if !types.ConvertibleTo(t2, t1) {
			return false
		}
	}
	return true
}

func resultsMatch(wanted, got []types.Type) bool {
	if len(got) == 0 {
		return true
	}
	if len(wanted) != len(got) {
		return false
	}
	for i, t1 := range wanted {
		t2 := got[i]
		if !types.ConvertibleTo(t2, t1) {
			return false
		}
	}
	return true
}

func interfaceMatching(calls map[string]call) string {
	matchesIface := func(decls map[string]funcDecl) bool {
		if len(calls) > len(decls) {
			return false
		}
		for n, d := range decls {
			c, e := calls[n]
			if !e {
				return false
			}
			if !paramsMatch(d.params, c.params) {
				return false
			}
			if !resultsMatch(d.results, c.results) {
				return false
			}
		}
		return true
	}
	for name, decls := range parsed {
		if matchesIface(decls) {
			return name
		}
	}
	return ""
}

func getDirs(d string) ([]string, error) {
	var dirs []string
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	}
	if err := filepath.Walk(d, walkFn); err != nil {
		return nil, err
	}
	return dirs, nil
}

func getPkgs(p string) ([]*build.Package, []string, error) {
	recursive := filepath.Base(p) == "..."
	if !recursive {
		info, err := os.Stat(p)
		if err != nil {
			return nil, nil, err
		}
		if !info.IsDir() {
			pkg := &build.Package{
				Name:    "stdin",
				GoFiles: []string{p},
			}
			return []*build.Package{pkg}, []string{"."}, nil
		}
	}
	d := p
	if recursive {
		d = p[:len(p)-4]
	}
	dirs, err := getDirs(d)
	if err != nil {
		return nil, nil, err
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	var pkgs []*build.Package
	var basedirs []string
	for _, d := range dirs {
		pkg, err := build.Import("./"+d, wd, 0)
		if _, ok := err.(*build.NoGoError); ok {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		pkgs = append(pkgs, pkg)
		basedirs = append(basedirs, d)
	}
	return pkgs, basedirs, nil
}

func checkPaths(paths []string, w io.Writer) error {
	conf := &types.Config{Importer: importer.Default()}
	for _, p := range paths {
		pkgs, basedirs, err := getPkgs(p)
		if err != nil {
			return err
		}
		for i, pkg := range pkgs {
			basedir := basedirs[i]
			if err := checkPkg(conf, pkg, basedir, w); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkPkg(conf *types.Config, pkg *build.Package, basedir string, w io.Writer) error {
	if *verbose {
		fmt.Fprintln(w, basedir)
	}
	gp := &goPkg{
		Package: pkg,
		fset:    token.NewFileSet(),
	}
	for _, p := range pkg.GoFiles {
		fp := filepath.Join(basedir, p)
		if err := gp.parsePath(fp); err != nil {
			return err
		}
	}
	if err := gp.check(conf, w); err != nil {
		return err
	}
	return nil
}

type goPkg struct {
	*build.Package

	fset  *token.FileSet
	files []*ast.File
}

func (gp *goPkg) parsePath(fp string) error {
	f, err := os.Open(fp)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := gp.parseReader(fp, f); err != nil {
		return err
	}
	return nil
}

func (gp *goPkg) parseReader(name string, r io.Reader) error {
	f, err := parser.ParseFile(gp.fset, name, r, 0)
	if err != nil {
		return err
	}
	gp.files = append(gp.files, f)
	return nil
}

func (gp *goPkg) check(conf *types.Config, w io.Writer) error {
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	_, err := conf.Check(gp.Name, gp.fset, gp.files, info)
	if err != nil {
		return err
	}

	v := &Visitor{
		Info: info,
		w:    w,
		fset: gp.fset,
	}
	for _, f := range gp.files {
		ast.Walk(v, f)
	}
	return nil
}

type Visitor struct {
	*types.Info

	w    io.Writer
	fset *token.FileSet

	nodes []ast.Node

	params map[string]types.Type
	used   map[string]map[string]call

	// TODO: don't just discard params with untracked usage
	unknown       map[string]struct{}
	recordUnknown bool
}

func typeMap(t *types.Tuple) map[string]types.Type {
	m := make(map[string]types.Type, t.Len())
	for i := 0; i < t.Len(); i++ {
		p := t.At(i)
		m[p.Name()] = p.Type()
	}
	return m
}

func (v *Visitor) Visit(node ast.Node) ast.Visitor {
	var top ast.Node
	if len(v.nodes) > 0 {
		top = v.nodes[len(v.nodes)-1]
	}
	switch x := node.(type) {
	case *ast.File:
	case *ast.FuncDecl:
		f := v.Defs[x.Name].(*types.Func)
		sign := f.Type().(*types.Signature)
		v.params = typeMap(sign.Params())
		v.used = make(map[string]map[string]call, 0)
		v.unknown = make(map[string]struct{})
	case *ast.CallExpr:
		if wasParamCall := v.onCall(x); wasParamCall {
			return nil
		}
	case *ast.BlockStmt:
		v.recordUnknown = true
	case *ast.Ident:
		if !v.recordUnknown {
			break
		}
		if _, e := v.params[x.Name]; e {
			v.unknown[x.Name] = struct{}{}
		}
	case nil:
		v.nodes = v.nodes[:len(v.nodes)-1]
		if _, ok := top.(*ast.FuncDecl); ok {
			v.funcEnded(top.Pos())
			v.params = nil
			v.used = nil
			v.unknown = nil
			v.recordUnknown = false
		}
	}
	if node != nil {
		v.nodes = append(v.nodes, node)
	}
	return v
}

func (v *Visitor) onCall(ce *ast.CallExpr) bool {
	if v.used == nil {
		return false
	}
	sel, ok := ce.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	left, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	vname := left.Name
	if _, e := v.params[vname]; !e {
		return false
	}
	sign := v.Types[ce.Fun].Type.(*types.Signature)
	c := call{}
	results := sign.Results()
	for i := 0; i < results.Len(); i++ {
		v := results.At(i)
		c.results = append(c.results, v.Type())
	}
	for _, a := range ce.Args {
		c.params = append(c.params, v.Types[a].Type)
	}
	if _, e := v.used[vname]; !e {
		v.used[vname] = make(map[string]call)
	}
	fname := sel.Sel.Name
	v.used[vname][fname] = c
	return true
}

func (v *Visitor) funcEnded(pos token.Pos) {
	for name, methods := range v.used {
		if _, e := v.unknown[name]; e {
			continue
		}
		iface := interfaceMatching(methods)
		if iface == "" {
			continue
		}
		param, e := v.params[name]
		if !e {
			continue
		}
		if iface == param.String() {
			continue
		}
		pos := v.fset.Position(pos)
		fmt.Fprintf(v.w, "%s:%d: %s can be %s\n",
			pos.Filename, pos.Line, name, iface)
	}
}

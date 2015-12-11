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
	params  []interface{}
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

func paramEqual(t1 types.Type, a2 interface{}) bool {
	switch x := a2.(type) {
	case types.Type:
		return types.ConvertibleTo(x, t1)
	case token.Token:
		return toToken[t1.String()] == x
	case nil:
		switch t1.(type) {
		case *types.Slice:
			return true
		case *types.Map:
			return true
		default:
			return false
		}
	default:
		panic("Unexpected param type")
		return false
	}
}

func paramsMatch(types1 []types.Type, args2 []interface{}) bool {
	if len(types1) != len(args2) {
		return false
	}
	for i, t1 := range types1 {
		a2 := args2[i]
		if !paramEqual(t1, a2) {
			return false
		}
	}
	return true
}

func resultEqual(t1 types.Type, e2 interface{}) bool {
	switch x := e2.(type) {
	case types.Type:
		return types.ConvertibleTo(x, t1)
	case nil:
		// assigning to _
		return true
	default:
		panic("Unexpected result type")
		return false
	}
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
		if !resultEqual(t1, t2) {
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
	pkg, err := conf.Check(gp.Name, gp.fset, gp.files, nil)
	if err != nil {
		return err
	}

	v := &Visitor{
		pkg:    gp.Package,
		w:      w,
		fset:   gp.fset,
		scopes: []*types.Scope{pkg.Scope()},
	}
	for _, f := range gp.files {
		ast.Walk(v, f)
	}
	return nil
}

type Visitor struct {
	pkg *build.Package

	w      io.Writer
	fset   *token.FileSet
	scopes []*types.Scope

	nodes []ast.Node

	params map[string]types.Type
	used   map[string]map[string]call

	// TODO: don't just discard params with untracked usage
	unknown       map[string]struct{}
	recordUnknown bool
}

func (v *Visitor) scope() *types.Scope {
	return v.scopes[len(v.scopes)-1]
}

func scopeName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return scopeName(x.X)
	default:
		return ""
	}
}

func (v *Visitor) funcType(fd *ast.FuncDecl) *types.Func {
	fname := fd.Name.Name
	f, ok := v.scope().Lookup(fname).(*types.Func)
	if !ok {
		return nil
	}
	return f
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
		f := v.funcType(x)
		if f == nil {
			return nil
		}
		v.scopes = append(v.scopes, f.Scope())
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
		fd, ok := top.(*ast.FuncDecl)
		if !ok {
			return nil
		}
		v.scopes = v.scopes[:len(v.scopes)-1]
		v.funcEnded(fd)
		v.params = nil
		v.used = nil
		v.unknown = nil
		v.recordUnknown = false
	}
	if node != nil {
		v.nodes = append(v.nodes, node)
	}
	return v
}

type methoder interface {
	NumMethods() int
	Method(i int) *types.Func
}

func extNamed(t types.Type) *types.Named {
	switch x := t.(type) {
	case *types.Named:
		return x
	case *types.Pointer:
		return extNamed(x.Elem())
	default:
		panic("Unexpected type")
	}
}

func methods(t types.Type) methoder {
	named := extNamed(t)
	if iface, ok := named.Underlying().(*types.Interface); ok {
		return iface
	}
	return named
}

func (v *Visitor) dType(t, f *ast.Ident) *types.Func {
	obj := v.scope().Lookup(t.Name)
	if obj == nil {
		return nil
	}
	found := methods(obj.Type())
	for i := 0; i < found.NumMethods(); i++ {
		m := found.Method(i)
		if m.Name() == f.Name {
			return m
		}
	}
	return nil
}

func (v *Visitor) getType(id *ast.Ident) types.Type {
	name := id.Name
	if name == "_" || name == "nil" {
		return nil
	}
	_, obj := v.scope().LookupParent(name, id.Pos())
	if obj == nil {
		panic("Could not find ident type")
	}
	switch x := obj.(type) {
	case *types.Var:
		return x.Type()
	case *types.Const:
		return x.Type()
	default:
		fmt.Printf("%T\n", x)
		panic("Unexpected object type")
	}
}

func (v *Visitor) descType(e ast.Expr) interface{} {
	switch x := e.(type) {
	case *ast.Ident:
		return v.getType(x)
	case *ast.BasicLit:
		return x.Kind
	default:
		return nil
	}
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
	right := sel.Sel
	vname := left.Name
	if _, e := v.params[vname]; !e {
		return false
	}
	c := call{}
	f := v.dType(left, right)
	if f == nil {
		return false
	}
	sign := f.Type().(*types.Signature)
	results := sign.Results()
	for i := 0; i < results.Len(); i++ {
		v := results.At(i)
		c.results = append(c.results, v.Type())
	}
	for _, a := range ce.Args {
		c.params = append(c.params, v.descType(a))
	}
	if _, e := v.used[vname]; !e {
		v.used[vname] = make(map[string]call)
	}
	fname := right.Name
	v.used[vname][fname] = c
	return true
}

func (v *Visitor) funcEnded(fd *ast.FuncDecl) {
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
		pos := v.fset.Position(fd.Pos())
		fmt.Fprintf(v.w, "%s:%d: %s can be %s\n",
			pos.Filename, pos.Line, name, iface)
	}
}

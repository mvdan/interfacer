// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/types"
)

// TODO: don't use a global state to allow concurrent use
var c *cache

func typesMatch(want, got []types.Type) bool {
	if len(want) != len(got) {
		return false
	}
	for i, w := range want {
		if !types.ConvertibleTo(got[i], w) {
			return false
		}
	}
	return true
}

func usedMatch(t types.Type, usedAs []types.Type) bool {
	for _, u := range usedAs {
		if !types.ConvertibleTo(t, u) {
			return false
		}
	}
	return true
}

func matchesIface(p *param, iface ifaceSign, canEmpty bool) bool {
	if len(p.calls) > len(iface.funcs) {
		return false
	}
	if !usedMatch(iface.t, p.usedAs) {
		return false
	}
	for name, f := range iface.funcs {
		c, e := p.calls[name]
		if !e {
			return canEmpty
		}
		if !typesMatch(f.params, c.params) {
			return false
		}
		if !typesMatch(f.results, c.results) {
			return false
		}
	}
	for _, to := range p.assigned {
		if to.discard {
			return false
		}
	}
	return true
}

func isFunc(sign *types.Signature, fsign funcSign) bool {
	params := sign.Params()
	if params.Len() != len(fsign.params) {
		return false
	}
	results := sign.Results()
	if results.Len() != len(fsign.results) {
		return false
	}
	for i, p := range fsign.params {
		ip := params.At(i).Type()
		if !types.AssignableTo(p, ip) {
			return false
		}
	}
	for i, r := range fsign.results {
		ir := results.At(i).Type()
		if !types.AssignableTo(r, ir) {
			return false
		}
	}
	return true
}

func implementsIface(sign *types.Signature) bool {
	for _, f := range c.funcs {
		if isFunc(sign, f) {
			return true
		}
	}
	return false
}

func (v *Visitor) fullPath(path, name string) string {
	if path == "" || path == v.Pkg.Path() {
		return name
	}
	return path + "." + name
}

func (v *Visitor) interfaceMatching(p *param) (string, *types.Interface) {
	for _, pkg := range pkgs {
		for _, iface := range c.pkgIfaces[pkg.path] {
			if matchesIface(p, iface, false) {
				return v.fullPath(pkg.path, iface.name), iface.t
			}
		}
	}
	for _, path := range c.curPaths {
		for _, iface := range c.pkgIfaces[path] {
			if matchesIface(p, iface, false) {
				return v.fullPath(path, iface.name), iface.t
			}
		}
	}
	return "", nil
}

func flattenImports(pkg *types.Package) []string {
	seen := make(map[string]bool)
	var paths []string
	var fromPkg func(*types.Package)
	fromPkg = func(pkg *types.Package) {
		path := pkg.Path()
		if seen[path] {
			return
		}
		if c.std[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
		for _, ipkg := range pkg.Imports() {
			fromPkg(ipkg)
		}
	}
	fromPkg(pkg)
	return paths
}

func orderedPkgs(prog *loader.Program, paths []string) []*loader.PackageInfo {
	if strings.HasSuffix(paths[0], ".go") {
		for _, pkg := range prog.InitialPackages() {
			path := pkg.Pkg.Path()
			if c.std[path] {
				continue
			}
			return []*loader.PackageInfo{pkg}
		}
	}
	var pkgs []*loader.PackageInfo
	for _, path := range paths {
		pkgs = append(pkgs, prog.Package(path))
	}
	return pkgs
}

func checkErrors(infos []*loader.PackageInfo) ([]*types.Package, error) {
	var pkgs []*types.Package
	for _, info := range infos {
		if info == nil {
			continue
		}
		if info.Errors != nil {
			return nil, info.Errors[0]
		}
		pkgs = append(pkgs, info.Pkg)
	}
	return pkgs, nil
}

func checkArgs(args []string, w io.Writer) error {
	paths, err := recurse(args)
	if err != nil {
		errExit(err)
	}
	if err := typesInit(); err != nil {
		return err
	}
	if _, err := c.FromArgs(paths, false); err != nil {
		return err
	}
	prog, err := c.Load()
	if err != nil {
		return err
	}
	pkgInfos := orderedPkgs(prog, paths)
	pkgs, err := checkErrors(pkgInfos)
	if err != nil {
		return err
	}
	typesGet(prog)
	for _, pkg := range pkgs {
		c.curPaths = flattenImports(pkg)
		info := prog.AllPackages[pkg]
		if err := checkPkg(&c.TypeChecker, info, w); err != nil {
			return err
		}
	}
	return nil
}

func checkPkg(conf *types.Config, pkg *loader.PackageInfo, w io.Writer) error {
	if *verbose {
		fmt.Fprintln(w, pkg.Pkg.Path())
	}
	v := &Visitor{
		PackageInfo: pkg,
		w:           w,
		fset:        c.Fset,
	}
	for _, f := range pkg.Files {
		ast.Walk(v, f)
	}
	return nil
}

type param struct {
	t types.Type

	calls   map[string]funcSign
	usedAs  []types.Type
	discard bool

	assigned []*param
}

type Visitor struct {
	*loader.PackageInfo

	w     io.Writer
	fset  *token.FileSet
	nodes []ast.Node

	params  map[string]*param
	extras  map[string]*param
	inBlock bool

	skipNext bool
}

func (v *Visitor) top() ast.Node {
	return v.nodes[len(v.nodes)-1]
}

func paramsMap(t *types.Tuple) map[string]*param {
	m := make(map[string]*param, t.Len())
	for i := 0; i < t.Len(); i++ {
		p := t.At(i)
		m[p.Name()] = &param{
			t:     p.Type().Underlying(),
			calls: make(map[string]funcSign),
		}
	}
	return m
}

func paramType(sign *types.Signature, i int) types.Type {
	params := sign.Params()
	extra := sign.Variadic() && i >= params.Len()-1
	if !extra {
		if i >= params.Len() {
			// builtins with multiple signatures
			return nil
		}
		return params.At(i).Type()
	}
	last := params.At(params.Len() - 1)
	switch x := last.Type().(type) {
	case *types.Slice:
		return x.Elem()
	default:
		return x
	}
}

func (v *Visitor) param(name string) *param {
	if p, e := v.params[name]; e {
		return p
	}
	if p, e := v.extras[name]; e {
		return p
	}
	p := &param{
		calls: make(map[string]funcSign),
	}
	v.extras[name] = p
	return p
}

func (v *Visitor) addUsed(name string, as types.Type) {
	if as == nil {
		return
	}
	p := v.param(name)
	p.usedAs = append(p.usedAs, as)
}

func (v *Visitor) addAssign(to, from string) {
	pto := v.param(to)
	pfrom := v.param(from)
	pfrom.assigned = append(pfrom.assigned, pto)
}

func (v *Visitor) discard(name string) {
	p := v.param(name)
	p.discard = true
}

func (v *Visitor) Visit(node ast.Node) ast.Visitor {
	if v.skipNext {
		v.skipNext = false
		return nil
	}
	switch x := node.(type) {
	case *ast.FuncDecl:
		sign := v.Defs[x.Name].Type().(*types.Signature)
		if implementsIface(sign) {
			return nil
		}
		v.params = paramsMap(sign.Params())
		v.extras = make(map[string]*param)
	case *ast.BlockStmt:
		if v.params != nil {
			v.inBlock = true
		}
	case *ast.SelectorExpr:
		if !v.inBlock {
			return nil
		}
		v.onSelector(x)
	case *ast.AssignStmt:
		for i, e := range x.Rhs {
			id, ok := e.(*ast.Ident)
			if !ok {
				continue
			}
			left := x.Lhs[i]
			v.addUsed(id.Name, v.Types[left].Type)
			if lid, ok := left.(*ast.Ident); ok {
				v.addAssign(lid.Name, id.Name)
			}
		}
	case *ast.CallExpr:
		if !v.inBlock {
			return nil
		}
		v.onCall(x)
		switch y := x.Fun.(type) {
		case *ast.Ident:
			v.skipNext = true
		case *ast.SelectorExpr:
			if _, ok := y.X.(*ast.Ident); ok {
				v.skipNext = true
			}
		}
	case nil:
		if fd, ok := v.top().(*ast.FuncDecl); ok {
			v.funcEnded(fd.Pos())
			v.params = nil
			v.extras = nil
			v.inBlock = false
		}
		v.nodes = v.nodes[:len(v.nodes)-1]
	}
	if node != nil {
		v.nodes = append(v.nodes, node)
	}
	return v
}

func funcSignature(t types.Type) *types.Signature {
	switch x := t.(type) {
	case *types.Signature:
		return x
	case *types.Named:
		return funcSignature(x.Underlying())
	default:
		return nil
	}
}

func (v *Visitor) onCall(ce *ast.CallExpr) {
	sign := funcSignature(v.Types[ce.Fun].Type)
	if sign == nil {
		return
	}
	for i, e := range ce.Args {
		if id, ok := e.(*ast.Ident); ok {
			v.addUsed(id.Name, paramType(sign, i))
		}
	}
	sel, ok := ce.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	left, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}
	p := v.param(left.Name)
	c := funcSign{}
	results := sign.Results()
	for i := 0; i < results.Len(); i++ {
		c.results = append(c.results, results.At(i).Type())
	}
	for _, a := range ce.Args {
		c.params = append(c.params, v.Types[a].Type)
	}
	p.calls[sel.Sel.Name] = c
	return
}

func (v *Visitor) onSelector(sel *ast.SelectorExpr) {
	if id, ok := sel.X.(*ast.Ident); ok {
		v.discard(id.Name)
	}
}

func (v *Visitor) funcEnded(pos token.Pos) {
	for name, p := range v.params {
		if p.discard {
			continue
		}
		ifname, iface := v.interfaceMatching(p)
		if iface == nil {
			continue
		}
		if types.AssignableTo(iface, p.t) {
			continue
		}
		pos := v.fset.Position(pos)
		fname := pos.Filename
		if fname[0] == '/' {
			fname = filepath.Join(v.Pkg.Path(), filepath.Base(fname))
		}
		fmt.Fprintf(v.w, "%s:%d: %s can be %s\n",
			fname, pos.Line, name, ifname)
	}
}

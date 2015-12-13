/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
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
	"regexp"
	"strings"
)

// TODO: don't use a global state to allow concurrent use
var c *cache

func typesMatch(wanted, got []types.Type) bool {
	if len(wanted) != len(got) {
		return false
	}
	for i, w := range wanted {
		g := got[i]
		if !types.ConvertibleTo(g, w) {
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
		if p.String() != ip.String() {
			return false
		}
	}
	for i, r := range fsign.results {
		ir := results.At(i).Type()
		if r.String() != ir.String() {
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

func fullPath(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

func interfaceMatching(p *param) (string, *types.Interface) {
	for path, ifaces := range c.stdIfaces {
		for _, iface := range ifaces {
			if matchesIface(p, iface, false) {
				return fullPath(path, iface.name), iface.t
			}
		}
	}
	for _, path := range c.curPaths {
		for _, iface := range c.pkgIfaces[path] {
			if matchesIface(p, iface, false) {
				return fullPath(path, iface.name), iface.t
			}
		}
	}
	return "", nil
}

var skipDir = regexp.MustCompile(`^(testdata|vendor|_.*|\.\+)$`)

func getDirs(d string) ([]string, error) {
	var dirs []string
	if d != "." && !strings.HasPrefix(d, "./") {
		return nil, fmt.Errorf("TODO: recursing into non-local import paths")
	}
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if skipDir.MatchString(info.Name()) {
			return filepath.SkipDir
		}
		if info.IsDir() {
			if !strings.HasPrefix(path, "./") {
				path = "./" + path
			}
			dirs = append(dirs, path)
		}
		return nil
	}
	if err := filepath.Walk(d, walkFn); err != nil {
		return nil, err
	}
	return dirs, nil
}

func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !info.IsDir(), nil
}

func getPkgs(paths []string) ([]*build.Package, []string, error) {
	ex, err := fileExists(paths[0])
	if err != nil {
		return nil, nil, err
	}
	if ex {
		pkg := &build.Package{
			Name:    ".",
			GoFiles: paths,
		}
		return []*build.Package{pkg}, []string{"."}, nil
	}
	var pkgs []*build.Package
	var basedirs []string
	wd, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	for _, p := range paths {
		var dirs []string
		recursive := filepath.Base(p) == "..."
		if !recursive {
			dirs = []string{p}
		} else {
			d := p[:len(p)-4]
			var err error
			dirs, err = getDirs(d)
			if err != nil {
				return nil, nil, err
			}
		}
		for _, d := range dirs {
			pkg, err := build.Import(d, wd, 0)
			if _, ok := err.(*build.NoGoError); ok {
				continue
			}
			if err != nil {
				return nil, nil, err
			}
			pkgs = append(pkgs, pkg)
			basedirs = append(basedirs, d)
		}
	}
	return pkgs, basedirs, nil
}

func checkPaths(paths []string, w io.Writer) error {
	if err := typesInit(); err != nil {
		return err
	}
	conf := &types.Config{Importer: importer.Default()}
	pkgs, basedirs, err := getPkgs(paths)
	if err != nil {
		return err
	}
	for i, pkg := range pkgs {
		basedir := basedirs[i]
		if err := checkPkg(conf, pkg, basedir, w); err != nil {
			return err
		}
	}
	return nil
}

func checkPkg(conf *types.Config, pkg *build.Package, basedir string, w io.Writer) error {
	if *verbose {
		importPath := pkg.ImportPath
		if importPath == "" {
			importPath = "command-line-arguments"
		}
		fmt.Fprintln(w, importPath)
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

func flattenImports(pkg *types.Package, impPath string) []string {
	seen := make(map[string]struct{})
	var paths []string
	var addPkg func(*types.Package, string)
	addPkg = func(pkg *types.Package, path string) {
		if _, e := seen[path]; e {
			return
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
		for _, ipkg := range pkg.Imports() {
			addPkg(ipkg, ipkg.Path())
		}
	}
	addPkg(pkg, impPath)
	return paths
}

func grabRecurse(pkg *types.Package, impPath string) {
	if _, e := c.done[impPath]; e {
		return
	}
	c.grabFromScope(pkg.Scope(), true, false, impPath)
	for _, ipkg := range pkg.Imports() {
		grabRecurse(ipkg, ipkg.Path())
	}
}

func (gp *goPkg) check(conf *types.Config, w io.Writer) error {
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	pkg, err := conf.Check(gp.Name, gp.fset, gp.files, info)
	if err != nil {
		return err
	}
	ownPath := gp.ImportPath
	c.curPaths = flattenImports(pkg, ownPath)
	grabRecurse(pkg, ownPath)
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

type param struct {
	t types.Type

	calls   map[string]funcSign
	usedAs  []types.Type
	discard bool

	assigned []*param
}

type Visitor struct {
	*types.Info

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
		id, ok := e.(*ast.Ident)
		if !ok {
			continue
		}
		v.addUsed(id.Name, paramType(sign, i))
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
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}
	v.discard(id.Name)
}

func (v *Visitor) funcEnded(pos token.Pos) {
	for name, p := range v.params {
		if p.discard {
			continue
		}
		ifname, iface := interfaceMatching(p)
		if iface == nil {
			continue
		}
		// TODO: re-enable without reactivating false positive
		// in alias.go
		// if types.ConvertibleTo(iface, p.t) {
		if iface.String() == p.t.String() {
			continue
		}
		pos := v.fset.Position(pos)
		fmt.Fprintf(v.w, "%s:%d: %s can be %s\n",
			pos.Filename, pos.Line, name, ifname)
	}
}

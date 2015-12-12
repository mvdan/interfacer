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
	"regexp"
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

func resultsMatch(wanted, got []types.Type) bool {
	if len(got) == 0 {
		return true
	}
	return typesMatch(wanted, got)
}

func usedMatch(t types.Type, usedAs []types.Type) bool {
	if len(usedAs) < 1 {
		return true
	}
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
		if !resultsMatch(f.results, c.results) {
			return false
		}
	}
	for _, to := range p.assigned {
		if to.discard {
			return false
		}
		if !matchesIface(to, iface, true) {
			return false
		}
	}
	return true
}

func interfaceMatching(p *param) string {
	for name, iface := range parsed {
		if matchesIface(p, iface, false) {
			return name
		}
	}
	return ""
}

var skipDir = regexp.MustCompile(`^(testdata|vendor|_.*)$`)

func getDirs(d string, recursive bool) ([]string, error) {
	var dirs []string
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if skipDir.MatchString(info.Name()) {
			return filepath.SkipDir
		}
		if info.IsDir() {
			dirs = append(dirs, path)
			if !recursive {
				return filepath.SkipDir
			}
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
	dirs, err := getDirs(d, recursive)
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
}

func (v *Visitor) top() ast.Node {
	if len(v.nodes) == 0 {
		return nil
	}
	return v.nodes[len(v.nodes)-1]
}

func paramsMap(t *types.Tuple) map[string]*param {
	m := make(map[string]*param, t.Len())
	for i := 0; i < t.Len(); i++ {
		p := t.At(i)
		m[p.Name()] = &param{
			t:     p.Type(),
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
	stype := params.At(params.Len() - 1).Type().(*types.Slice)
	return stype.Elem()
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
	switch x := node.(type) {
	case *ast.FuncDecl:
		sign := v.Defs[x.Name].Type().(*types.Signature)
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
		if !v.inBlock {
			return nil
		}
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
	if _, ok := v.top().(*ast.CallExpr); ok {
		return
	}
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
		iface := interfaceMatching(p)
		if iface == "" {
			continue
		}
		if iface == p.t.String() {
			continue
		}
		pos := v.fset.Position(pos)
		fmt.Fprintf(v.w, "%s:%d: %s can be %s\n",
			pos.Filename, pos.Line, name, iface)
	}
}

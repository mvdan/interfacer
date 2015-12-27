// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package interfacer

import (
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/types"
)

func (v *visitor) implementsIface(sign *types.Signature) bool {
	s := signString(sign)
	return v.funcOf(s) != ""
}

func doMethoderType(t types.Type) map[string]string {
	switch x := t.(type) {
	case *types.Pointer:
		return doMethoderType(x.Elem())
	case *types.Named:
		if u, ok := x.Underlying().(*types.Interface); ok {
			return doMethoderType(u)
		}
		return namedMethodMap(x)
	case *types.Interface:
		return ifaceFuncMap(x)
	default:
		return nil
	}
}

func assignable(s, t string, called, want map[string]string) bool {
	if s == t {
		return true
	}
	if len(t) >= len(s) {
		return false
	}
	for fname, ftype := range want {
		s, e := called[fname]
		if !e || s != ftype {
			return false
		}
	}
	return true
}

func toDiscard(vr *variable) bool {
	if vr.discard {
		return true
	}
	for to := range vr.assigned {
		if toDiscard(to) {
			return true
		}
	}
	return false
}

func (v *visitor) interfaceMatching(obj types.Object, vr *variable) (string, string) {
	if toDiscard(vr) {
		return "", ""
	}
	allFuncs := doMethoderType(obj.Type())
	called := make(map[string]string, len(vr.calls))
	for fname := range vr.calls {
		called[fname] = allFuncs[fname]
	}
	s := funcMapString(called)
	name := v.ifaceOf(s)
	if name == "" {
		return "", ""
	}
	for t := range vr.usedAs {
		iface, ok := t.(*types.Interface)
		if !ok {
			return "", ""
		}
		asMethods := ifaceFuncMap(iface)
		as := funcMapString(asMethods)
		if !assignable(s, as, called, asMethods) {
			return "", ""
		}
	}
	return name, s
}

func orderedPkgs(prog *loader.Program) ([]*types.Package, error) {
	// TODO: InitialPackages() is not in the order that we passed to
	// it via Import() calls.
	// For now, make it deterministic by sorting by import path.
	unordered := prog.InitialPackages()
	paths := make([]string, 0, len(unordered))
	byPath := make(map[string]*types.Package, len(unordered))
	for _, info := range unordered {
		if info.Errors != nil {
			return nil, info.Errors[0]
		}
		path := info.Pkg.Path()
		paths = append(paths, path)
		byPath[path] = info.Pkg
	}
	sort.Sort(ByAlph(paths))
	pkgs := make([]*types.Package, 0, len(unordered))
	for _, path := range paths {
		pkgs = append(pkgs, byPath[path])
	}
	return pkgs, nil
}

// relPathErr makes converts errors by go/types and go/loader that use
// absolute paths into errors with relative paths
func relPathErr(err error) error {
	errStr := fmt.Sprintf("%v", err)
	if !strings.HasPrefix(errStr, "/") {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	if strings.HasPrefix(errStr, wd) {
		return fmt.Errorf(errStr[len(wd)+1:])
	}
	return err
}

func CheckArgs(args []string, w io.Writer, verbose bool) error {
	paths, err := recurse(args)
	if err != nil {
		return err
	}
	c := newCache()
	if _, err := c.FromArgs(paths, false); err != nil {
		return err
	}
	prog, err := c.Load()
	if err != nil {
		return err
	}
	pkgs, err := orderedPkgs(prog)
	if err != nil {
		return relPathErr(err)
	}
	c.typesGet(pkgs)
	for _, pkg := range pkgs {
		info := prog.AllPackages[pkg]
		if verbose {
			fmt.Fprintln(w, info.Pkg.Path())
		}
		checkPkg(c, info, prog.Fset, w)
	}
	return nil
}

func checkPkg(c *cache, info *loader.PackageInfo, fset *token.FileSet, w io.Writer) {
	v := &visitor{
		cache:       c,
		PackageInfo: info,
		w:           w,
		fset:        fset,
		vars:        make(map[types.Object]*variable),
	}
	for _, f := range info.Files {
		ast.Walk(v, f)
	}
}

type variable struct {
	calls   map[string]struct{}
	usedAs  map[types.Type]struct{}
	discard bool

	assigned map[*variable]struct{}
}

type visitor struct {
	*cache
	*loader.PackageInfo

	w     io.Writer
	fset  *token.FileSet
	signs []*types.Signature

	vars map[types.Object]*variable

	skipNext bool
}

func (v *visitor) addParams(t *types.Tuple) {
	for i := 0; i < t.Len(); i++ {
		obj := t.At(i)
		v.vars[obj] = &variable{
			calls:    make(map[string]struct{}),
			usedAs:   make(map[types.Type]struct{}),
			assigned: make(map[*variable]struct{}),
		}
	}
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

func (v *visitor) variable(id *ast.Ident) *variable {
	obj := v.ObjectOf(id)
	if vr, e := v.vars[obj]; e {
		return vr
	}
	vr := &variable{
		calls:    make(map[string]struct{}),
		usedAs:   make(map[types.Type]struct{}),
		assigned: make(map[*variable]struct{}),
	}
	v.vars[obj] = vr
	return vr
}

func (v *visitor) addUsed(id *ast.Ident, as types.Type) {
	if as == nil {
		return
	}
	vr := v.variable(id)
	vr.usedAs[as.Underlying()] = struct{}{}
}

func (v *visitor) addAssign(to, from *ast.Ident) {
	pto := v.variable(to)
	pfrom := v.variable(from)
	pfrom.assigned[pto] = struct{}{}
}

func (v *visitor) discard(e ast.Expr) {
	id, ok := e.(*ast.Ident)
	if !ok {
		return
	}
	vr := v.variable(id)
	vr.discard = true
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
	if v.skipNext {
		v.skipNext = false
		return nil
	}
	var sign *types.Signature
	switch x := node.(type) {
	case *ast.FuncLit:
		sign = v.Types[x].Type.(*types.Signature)
		if v.implementsIface(sign) {
			return nil
		}
		v.addParams(sign.Params())
	case *ast.FuncDecl:
		sign = v.Defs[x.Name].Type().(*types.Signature)
		if v.implementsIface(sign) {
			return nil
		}
		v.addParams(sign.Params())
	case *ast.SelectorExpr:
		v.discard(x.X)
	case *ast.UnaryExpr:
		v.discard(x.X)
	case *ast.BinaryExpr:
		v.discard(x.X)
		v.discard(x.Y)
	case *ast.IndexExpr:
		v.discard(x.X)
	case *ast.IncDecStmt:
		v.discard(x.X)
	case *ast.AssignStmt:
		v.onAssign(x)
	case *ast.CallExpr:
		v.onCall(x)
	case nil:
		top := v.signs[len(v.signs)-1]
		if top != nil {
			v.funcEnded(top)
		}
		v.signs = v.signs[:len(v.signs)-1]
	}
	if node != nil {
		v.signs = append(v.signs, sign)
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

func (v *visitor) onAssign(as *ast.AssignStmt) {
	for i, e := range as.Rhs {
		id, ok := e.(*ast.Ident)
		if !ok {
			continue
		}
		left := as.Lhs[i]
		v.addUsed(id, v.Types[left].Type)
		if lid, ok := left.(*ast.Ident); ok {
			v.addAssign(lid, id)
		}
	}
}

func (v *visitor) onCall(ce *ast.CallExpr) {
	switch y := ce.Fun.(type) {
	case *ast.Ident:
		v.skipNext = true
	case *ast.SelectorExpr:
		if _, ok := y.X.(*ast.Ident); ok {
			v.skipNext = true
		}
	}
	sign := funcSignature(v.Types[ce.Fun].Type)
	if sign == nil {
		return
	}
	for i, e := range ce.Args {
		if id, ok := e.(*ast.Ident); ok {
			v.addUsed(id, paramType(sign, i))
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
	vr := v.variable(left)
	vr.calls[sel.Sel.Name] = struct{}{}
	return
}

func (v *visitor) funcEnded(sign *types.Signature) {
	params := sign.Params()
	for i := 0; i < params.Len(); i++ {
		obj := params.At(i)
		vr := v.vars[obj]
		v.evalParam(obj, vr)
	}
}

func (v *visitor) evalParam(obj types.Object, vr *variable) {
	ifname, iftype := v.interfaceMatching(obj, vr)
	if ifname == "" {
		return
	}
	t := obj.Type()
	if _, haveIface := t.Underlying().(*types.Interface); haveIface {
		if ifname == t.String() {
			return
		}
		have := funcMapString(doMethoderType(t))
		if have == iftype {
			return
		}
	}
	pos := v.fset.Position(obj.Pos())
	fname := pos.Filename
	if fname[0] == '/' {
		fname = filepath.Join(v.Pkg.Path(), filepath.Base(fname))
	}
	pname := v.Pkg.Name()
	if strings.HasPrefix(ifname, pname+".") {
		ifname = ifname[len(pname)+1:]
	}
	fmt.Fprintf(v.w, "%s:%d:%d: %s can be %s\n",
		fname, pos.Line, pos.Column, obj.Name(), ifname)
}

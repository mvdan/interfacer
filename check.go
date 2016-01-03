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

func toDiscard(vu *varUsage) bool {
	if vu.discard {
		return true
	}
	for to := range vu.assigned {
		if toDiscard(to) {
			return true
		}
	}
	return false
}

func (v *visitor) interfaceMatching(vr *types.Var, vu *varUsage) (string, string) {
	if toDiscard(vu) {
		return "", ""
	}
	allFuncs := typeFuncMap(vr.Type())
	called := make(map[string]string, len(vu.calls))
	for fname := range vu.calls {
		called[fname] = allFuncs[fname]
	}
	s := funcMapString(called)
	name := v.ifaceOf(s)
	if name == "" {
		return "", ""
	}
	return name, s
}

func orderedPkgs(prog *loader.Program) ([]*types.Package, error) {
	// InitialPackages() is not in the order that we passed to it
	// via Import() calls.
	// For now, make it deterministic by sorting import paths
	// alphabetically.
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
func relPathErr(err error, wd string) error {
	errStr := fmt.Sprintf("%v", err)
	if !strings.HasPrefix(errStr, "/") {
		return err
	}
	if strings.HasPrefix(errStr, wd) {
		return fmt.Errorf(errStr[len(wd)+1:])
	}
	return err
}

func CheckArgs(args []string, w io.Writer, verbose bool) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
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
		return relPathErr(err, wd)
	}
	c.typesGet(pkgs)
	for _, pkg := range pkgs {
		info := prog.AllPackages[pkg]
		if verbose {
			fmt.Fprintln(w, info.Pkg.Path())
		}
		v := &visitor{
			cache:       c,
			PackageInfo: info,
			wd:          wd,
			w:           w,
			fset:        prog.Fset,
			vars:        make(map[*types.Var]*varUsage),
		}
		for _, f := range info.Files {
			ast.Walk(v, f)
		}
	}
	return nil
}

type varUsage struct {
	calls   map[string]struct{}
	discard bool

	assigned map[*varUsage]struct{}
}

type visitor struct {
	*cache
	*loader.PackageInfo

	wd    string
	w     io.Writer
	fset  *token.FileSet
	signs []*types.Signature
	warns [][]string
	level int

	vars map[*types.Var]*varUsage

	skipNext bool
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

func (v *visitor) varUsage(id *ast.Ident) *varUsage {
	vr, ok := v.ObjectOf(id).(*types.Var)
	if !ok {
		return nil
	}
	if vu, e := v.vars[vr]; e {
		return vu
	}
	vu := &varUsage{
		calls:    make(map[string]struct{}),
		assigned: make(map[*varUsage]struct{}),
	}
	v.vars[vr] = vu
	return vu
}

func (v *visitor) addUsed(id *ast.Ident, as types.Type) {
	if as == nil {
		return
	}
	vu := v.varUsage(id)
	if vu == nil {
		// not a variable
		return
	}
	iface, ok := as.Underlying().(*types.Interface)
	if !ok {
		vu.discard = true
		return
	}
	for i := 0; i < iface.NumMethods(); i++ {
		m := iface.Method(i)
		vu.calls[m.Name()] = struct{}{}
	}
}

func (v *visitor) addAssign(to, from *ast.Ident) {
	pto := v.varUsage(to)
	pfrom := v.varUsage(from)
	if pto == nil || pfrom == nil {
		// either isn't a variable
		return
	}
	pfrom.assigned[pto] = struct{}{}
}

func (v *visitor) discard(e ast.Expr) {
	id, ok := e.(*ast.Ident)
	if !ok {
		return
	}
	vu := v.varUsage(id)
	if vu == nil {
		// not a variable
		return
	}
	vu.discard = true
}

func (v *visitor) comparedWith(e ast.Expr, with ast.Expr) {
	if _, ok := with.(*ast.BasicLit); ok {
		v.discard(e)
	}
}

func (v *visitor) implementsIface(sign *types.Signature) bool {
	s := signString(sign)
	return v.funcOf(s) != ""
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
	case *ast.FuncDecl:
		sign = v.Defs[x.Name].Type().(*types.Signature)
		if v.implementsIface(sign) {
			return nil
		}
	case *ast.SelectorExpr:
		v.discard(x.X)
	case *ast.UnaryExpr:
		v.discard(x.X)
	case *ast.IndexExpr:
		v.discard(x.X)
	case *ast.IncDecStmt:
		v.discard(x.X)
	case *ast.BinaryExpr:
		v.onBinary(x)
	case *ast.AssignStmt:
		v.onAssign(x)
	case *ast.KeyValueExpr:
		v.onKeyValue(x)
	case *ast.CompositeLit:
		v.onComposite(x)
	case *ast.CallExpr:
		v.onCall(x)
	case nil:
		if top := v.signs[len(v.signs)-1]; top != nil {
			v.funcEnded(top)
		}
		v.signs = v.signs[:len(v.signs)-1]
	}
	if node != nil {
		v.signs = append(v.signs, sign)
		if sign != nil {
			v.level++
		}
	}
	return v
}

func (v *visitor) onBinary(be *ast.BinaryExpr) {
	switch be.Op {
	case token.EQL, token.NEQ:
	default:
		v.discard(be.X)
		v.discard(be.Y)
		return
	}
	v.comparedWith(be.X, be.Y)
	v.comparedWith(be.Y, be.X)
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

func (v *visitor) onKeyValue(kv *ast.KeyValueExpr) {
	if id, ok := kv.Key.(*ast.Ident); ok {
		v.addUsed(id, v.TypeOf(kv.Value))
	}
	if id, ok := kv.Value.(*ast.Ident); ok {
		v.addUsed(id, v.TypeOf(kv.Key))
	}
}

func (v *visitor) onComposite(cl *ast.CompositeLit) {
	for _, e := range cl.Elts {
		if kv, ok := e.(*ast.KeyValueExpr); ok {
			v.onKeyValue(kv)
		}
	}
}

func (v *visitor) onCall(ce *ast.CallExpr) {
	if sign, ok := v.TypeOf(ce.Fun).(*types.Signature); ok {
		v.onMethodCall(ce, sign)
		return
	}
	if len(ce.Args) == 1 {
		v.discard(ce.Args[0])
	}
}

func (v *visitor) onMethodCall(ce *ast.CallExpr, sign *types.Signature) {
	switch y := ce.Fun.(type) {
	case *ast.Ident:
		v.skipNext = true
	case *ast.SelectorExpr:
		if _, ok := y.X.(*ast.Ident); ok {
			v.skipNext = true
		}
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
	vu := v.varUsage(left)
	if vu == nil {
		// not a variable
		return
	}
	vu.calls[sel.Sel.Name] = struct{}{}
}

func (v *visitor) funcEnded(sign *types.Signature) {
	v.level--
	v.warns = append(v.warns, v.funcWarns(sign))
	if v.level > 0 {
		return
	}
	for i := len(v.warns) - 1; i >= 0; i-- {
		warns := v.warns[i]
		for _, warn := range warns {
			fmt.Fprintln(v.w, warn)
		}
	}
	v.warns = nil
	v.vars = make(map[*types.Var]*varUsage)
}

func (v *visitor) funcWarns(sign *types.Signature) []string {
	var warns []string
	params := sign.Params()
	for i := 0; i < params.Len(); i++ {
		vr := params.At(i)
		vu := v.vars[vr]
		if vu == nil {
			continue
		}
		if warn := v.paramWarn(vr, vu); warn != "" {
			warns = append(warns, warn)
		}
	}
	return warns
}

func (v *visitor) paramWarn(vr *types.Var, vu *varUsage) string {
	ifname, iftype := v.interfaceMatching(vr, vu)
	if ifname == "" {
		return ""
	}
	t := vr.Type()
	if _, haveIface := t.Underlying().(*types.Interface); haveIface {
		if ifname == t.String() {
			return ""
		}
		have := funcMapString(typeFuncMap(t))
		if have == iftype {
			return ""
		}
	}
	pos := v.fset.Position(vr.Pos())
	fname := pos.Filename
	// go/loader seems to like absolute paths
	if rel, err := filepath.Rel(v.wd, fname); err == nil {
		fname = rel
	}
	pname := v.Pkg.Name()
	if strings.HasPrefix(ifname, "./") {
		ifname = ifname[2:]
	}
	if strings.HasPrefix(ifname, pname+".") {
		ifname = ifname[len(pname)+1:]
	}
	return fmt.Sprintf("%s:%d:%d: %s can be %s",
		fname, pos.Line, pos.Column, vr.Name(), ifname)
}

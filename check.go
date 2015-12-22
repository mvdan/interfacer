// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package interfacer

import (
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/types"
)

// TODO: don't use a global state to allow concurrent use
var c *cache

func implementsIface(sign *types.Signature) bool {
	s := signString(sign)
	_, e := funcs[s]
	return e
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

func interfaceMatching(p *param) (string, string) {
	for to := range p.assigned {
		if to.discard {
			return "", ""
		}
	}
	allFuncs := doMethoderType(p.t)
	called := make(map[string]string, len(p.calls))
	for fname := range p.calls {
		called[fname] = allFuncs[fname]
	}
	s := funcMapString(called)
	name, e := ifaces[s]
	if !e {
		return "", ""
	}
	for t := range p.usedAs {
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
	var paths []string
	for _, info := range prog.InitialPackages() {
		if info.Errors != nil {
			return nil, info.Errors[0]
		}
		paths = append(paths, info.Pkg.Path())
	}
	sort.Sort(ByAlph(paths))
	var pkgs []*types.Package
	for _, path := range paths {
		info := prog.Package(path)
		pkgs = append(pkgs, info.Pkg)
	}
	return pkgs, nil
}

func CheckArgs(args []string, w io.Writer, verbose bool) error {
	paths, err := recurse(args)
	if err != nil {
		return err
	}
	typesInit(paths)
	if _, err := c.FromArgs(paths, false); err != nil {
		return err
	}
	prog, err := c.Load()
	if err != nil {
		return err
	}
	pkgs, err := orderedPkgs(prog)
	if err != nil {
		return err
	}
	typesGet(pkgs)
	for _, pkg := range pkgs {
		info := prog.AllPackages[pkg]
		if verbose {
			fmt.Fprintln(w, info.Pkg.Path())
		}
		checkPkg(&c.TypeChecker, info, w)
	}
	return nil
}

func checkPkg(conf *types.Config, info *loader.PackageInfo, w io.Writer) {
	v := &visitor{
		PackageInfo: info,
		w:           w,
		fset:        c.Fset,
	}
	for _, f := range info.Files {
		ast.Walk(v, f)
	}
}

type param struct {
	t   types.Type
	pos token.Pos

	calls   map[string]struct{}
	usedAs  map[types.Type]struct{}
	discard bool

	assigned map[*param]struct{}
}

type visitor struct {
	*loader.PackageInfo

	w     io.Writer
	fset  *token.FileSet
	nodes []ast.Node

	params  map[string]*param
	extras  map[string]*param
	inBlock bool

	skipNext bool
}

func (v *visitor) top() ast.Node {
	return v.nodes[len(v.nodes)-1]
}

func paramsMap(t *types.Tuple) map[string]*param {
	m := make(map[string]*param, t.Len())
	for i := 0; i < t.Len(); i++ {
		p := t.At(i)
		m[p.Name()] = &param{
			t:        p.Type(),
			pos:      p.Pos(),
			calls:    make(map[string]struct{}),
			usedAs:   make(map[types.Type]struct{}),
			assigned: make(map[*param]struct{}),
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

func (v *visitor) param(name string) *param {
	if p, e := v.params[name]; e {
		return p
	}
	if p, e := v.extras[name]; e {
		return p
	}
	p := &param{
		calls:    make(map[string]struct{}),
		usedAs:   make(map[types.Type]struct{}),
		assigned: make(map[*param]struct{}),
	}
	v.extras[name] = p
	return p
}

func (v *visitor) addUsed(name string, as types.Type) {
	if as == nil {
		return
	}
	p := v.param(name)
	p.usedAs[as.Underlying()] = struct{}{}
}

func (v *visitor) addAssign(to, from string) {
	pto := v.param(to)
	pfrom := v.param(from)
	pfrom.assigned[pto] = struct{}{}
}

func (v *visitor) discard(name string) {
	p := v.param(name)
	p.discard = true
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
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
		switch y := x.Fun.(type) {
		case *ast.Ident:
			v.skipNext = true
		case *ast.SelectorExpr:
			if _, ok := y.X.(*ast.Ident); ok {
				v.skipNext = true
			}
		}
	case nil:
		if _, ok := v.top().(*ast.FuncDecl); ok {
			v.funcEnded()
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

func (v *visitor) onCall(ce *ast.CallExpr) {
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
	p.calls[sel.Sel.Name] = struct{}{}
	return
}

func (v *visitor) onSelector(sel *ast.SelectorExpr) {
	if id, ok := sel.X.(*ast.Ident); ok {
		v.discard(id.Name)
	}
}

func (v *visitor) funcEnded() {
	for name, p := range v.params {
		if p.discard {
			continue
		}
		ifname, iftype := interfaceMatching(p)
		if ifname == "" {
			continue
		}
		if _, haveIface := p.t.Underlying().(*types.Interface); haveIface {
			if ifname == p.t.String() {
				continue
			}
			have := funcMapString(doMethoderType(p.t))
			if have == iftype {
				continue
			}
		}
		pos := v.fset.Position(p.pos)
		fname := pos.Filename
		if fname[0] == '/' {
			fname = filepath.Join(v.Pkg.Path(), filepath.Base(fname))
		}
		pname := v.Pkg.Name()
		if strings.HasPrefix(ifname, pname+".") {
			ifname = ifname[len(pname)+1:]
		}
		fmt.Fprintf(v.w, "%s:%d:%d: %s can be %s\n",
			fname, pos.Line, pos.Column, name, ifname)
	}
}

// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package interfacer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/loader"
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
	if allFuncs == nil {
		return "", ""
	}
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

// relPathErr converts errors by go/types and go/loader that use
// absolute paths into errors with relative paths
func relPathErr(err error, wd string) error {
	errStr := fmt.Sprintf("%v", err)
	if strings.HasPrefix(errStr, wd) {
		return fmt.Errorf(errStr[len(wd)+1:])
	}
	return err
}

// CheckArgs checks the packages specified by their import paths in
// args, and writes the results in w. Can give verbose output if
// specified, printing each package as it is checked.
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
	rest, err := c.FromArgs(paths, false)
	if err != nil {
		return err
	}
	if len(rest) > 0 {
		return fmt.Errorf("unwanted extra args: %v", rest)
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
	v := &visitor{
		cache: c,
		wd:    wd,
		w:     w,
		fset:  prog.Fset,
	}
	for _, pkg := range pkgs {
		if verbose {
			fmt.Fprintln(w, pkg.Path())
		}
		info := prog.AllPackages[pkg]
		v.PackageInfo = info
		v.vars = make(map[*types.Var]*varUsage)
		v.impNames = make(map[string]string)
		for _, f := range info.Files {
			for _, imp := range f.Imports {
				if imp.Name == nil {
					continue
				}
				name := imp.Name.Name
				path, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					return err
				}
				v.impNames[path] = name
			}
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

type funcDecl struct {
	name string
	sign *types.Signature
}

type visitor struct {
	*cache
	*loader.PackageInfo

	wd    string
	w     io.Writer
	fset  *token.FileSet
	funcs []*funcDecl
	warns [][]string
	level int

	vars     map[*types.Var]*varUsage
	impNames map[string]string
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
	var sign *types.Signature
	var name string
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
		name = x.Name.Name
	case *ast.SelectorExpr:
		if _, ok := v.TypeOf(x.Sel).(*types.Signature); !ok {
			v.discard(x.X)
		}
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
		if top := v.funcs[len(v.funcs)-1]; top != nil {
			v.funcEnded(top)
		}
		v.funcs = v.funcs[:len(v.funcs)-1]
	}
	if node != nil {
		if sign != nil {
			v.funcs = append(v.funcs, &funcDecl{
				sign: sign,
				name: name,
			})
			v.level++
		} else {
			v.funcs = append(v.funcs, nil)
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
		switch x := e.(type) {
		case *ast.KeyValueExpr:
			v.onKeyValue(x)
		case *ast.Ident:
			t := v.TypeOf(cl.Type).Underlying().(*types.Struct)
			if t.NumFields() != 1 {
				panic("expected exactly one field")
			}
			ft := t.Field(0).Type()
			v.addUsed(x, ft)
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

func (v *visitor) funcEnded(fd *funcDecl) {
	v.level--
	v.warns = append(v.warns, v.funcWarns(fd))
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

func (v *visitor) funcWarns(fd *funcDecl) []string {
	var warns []string
	params := fd.sign.Params()
	for i := 0; i < params.Len(); i++ {
		vr := params.At(i)
		vu := v.vars[vr]
		if vu == nil {
			continue
		}
		warn := v.paramWarn(fd.name, vr, vu)
		if warn == "" {
			continue
		}
		pos := v.fset.Position(vr.Pos())
		fname := pos.Filename
		// go/loader seems to like absolute paths
		if rel, err := filepath.Rel(v.wd, fname); err == nil {
			fname = rel
		}
		warns = append(warns, fmt.Sprintf("%s:%d:%d: %s",
			fname, pos.Line, pos.Column, warn))
	}
	return warns
}

var fullPathParts = regexp.MustCompile(`^(\*)?(([^/]+/)*([^/]+)\.)?([^/]+)$`)

func (v *visitor) simpleName(fullName string) string {
	pname := v.Pkg.Path()
	if strings.HasPrefix(fullName, pname+".") {
		return fullName[len(pname)+1:]
	}
	ps := fullPathParts.FindStringSubmatch(fullName)
	fullPkg := strings.TrimSuffix(ps[2], ".")
	star := ps[1]
	pkg := ps[4]
	if name, e := v.impNames[fullPkg]; e {
		pkg = name
	}
	name := ps[5]
	return star + pkg + "." + name
}

func (v *visitor) paramWarn(funcName string, vr *types.Var, vu *varUsage) string {
	t := vr.Type()
	named := typeNamed(t)
	if named != nil {
		name := named.Obj().Name()
		if mentionsType(funcName, name) {
			return ""
		}
	}
	ifname, iftype := v.interfaceMatching(vr, vu)
	if ifname == "" {
		return ""
	}
	if _, ok := t.Underlying().(*types.Interface); ok {
		if ifname == t.String() {
			return ""
		}
		if have := funcMapString(typeFuncMap(t)); have == iftype {
			return ""
		}
	}
	return fmt.Sprintf("%s can be %s", vr.Name(), v.simpleName(ifname))
}

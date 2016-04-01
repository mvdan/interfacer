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
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/loader"
)

func toDiscard(usage *varUsage) bool {
	if usage.discard {
		return true
	}
	for to := range usage.assigned {
		if toDiscard(to) {
			return true
		}
	}
	return false
}

func (v *visitor) interfaceMatching(param *types.Var, usage *varUsage) (string, string) {
	if toDiscard(usage) {
		return "", ""
	}
	allFuncs := typeFuncMap(param.Type())
	if allFuncs == nil {
		return "", ""
	}
	called := make(map[string]string, len(usage.calls))
	for fname := range usage.calls {
		called[fname] = allFuncs[fname]
	}
	s := funcMapString(called)
	name := v.ifaceOf(s)
	if name == "" {
		return "", ""
	}
	return name, s
}

func progPackages(prog *loader.Program) ([]*types.Package, error) {
	// InitialPackages() is not in the order that we passed to it
	// via Import() calls.
	// For now, make it deterministic by sorting import paths
	// alphabetically.
	unordered := prog.InitialPackages()
	paths := make([]string, 0, len(unordered))
	for _, info := range unordered {
		if info.Errors != nil {
			return nil, info.Errors[0]
		}
		paths = append(paths, info.Pkg.Path())
	}
	sort.Sort(ByAlph(paths))
	pkgs := make([]*types.Package, 0, len(unordered))
	for _, path := range paths {
		pkgs = append(pkgs, prog.Package(path).Pkg)
	}
	return pkgs, nil
}

// relPathErr converts errors by go/types and go/loader that use
// absolute paths into errors with relative paths.
func relPathErr(err error, wd string) error {
	errStr := fmt.Sprintf("%v", err)
	if strings.HasPrefix(errStr, wd) {
		return fmt.Errorf(errStr[len(wd)+1:])
	}
	return err
}

// Warn is an interfacer warning suggesting a better type for a function
// parameter.
type Warn struct {
	// Position and name of the parameter
	Pos  token.Position
	Name string
	// New suggested type
	Type string
}

func (w Warn) String() string {
	return fmt.Sprintf("%s:%d:%d: %s can be %s",
		w.Pos.Filename, w.Pos.Line, w.Pos.Column, w.Name, w.Type)
}

type varUsage struct {
	calls   map[string]struct{}
	discard bool

	assigned map[*varUsage]struct{}
}

type funcDecl struct {
	name    string
	sign    *types.Signature
	astType *ast.FuncType
}

type visitor struct {
	*cache
	*loader.PackageInfo

	wd    string
	fset  *token.FileSet
	funcs []*funcDecl
	warns []Warn

	vars     map[*types.Var]*varUsage
	impNames map[string]string
}

// CheckArgs checks the packages specified by their import paths in
// args. It will call the onWarns function as each package is processed,
// passing its import path and the warnings found. Returns an error, if
// any.
func CheckArgs(args []string, onWarns func(string, []Warn)) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	paths, err := recurse(args...)
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
	pkgs, err := progPackages(prog)
	if err != nil {
		return relPathErr(err, wd)
	}
	c.typesGet(pkgs)
	v := &visitor{
		cache: c,
		wd:    wd,
		fset:  prog.Fset,
	}
	for _, pkg := range pkgs {
		warns := v.checkPkg(prog.AllPackages[pkg])
		onWarns(pkg.Path(), warns)
	}
	return nil
}

// CheckArgsList is like CheckArgs, but returning a list of all the
// warnings instead.
func CheckArgsList(args []string) ([]Warn, error) {
	var all []Warn
	onWarns := func(path string, warns []Warn) {
		all = append(all, warns...)
	}
	if err := CheckArgs(args, onWarns); err != nil {
		return nil, err
	}
	return all, nil
}

// CheckArgsOutput is like CheckArgs, but intended for human-readable
// text output.
func CheckArgsOutput(args []string, w io.Writer, verbose bool) error {
	onWarns := func(path string, warns []Warn) {
		if verbose {
			fmt.Fprintln(w, path)
		}
		for _, warn := range warns {
			fmt.Fprintln(w, warn.String())
		}
	}
	return CheckArgs(args, onWarns)
}

func (v *visitor) checkPkg(info *loader.PackageInfo) []Warn {
	v.PackageInfo = info
	v.warns = nil
	v.vars = make(map[*types.Var]*varUsage)
	v.impNames = make(map[string]string)
	for _, f := range info.Files {
		for _, imp := range f.Imports {
			if imp.Name == nil {
				continue
			}
			name := imp.Name.Name
			path, _ := strconv.Unquote(imp.Path.Value)
			v.impNames[path] = name
		}
		ast.Walk(v, f)
	}
	sort.Sort(byPos(v.warns))
	return v.warns
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

func (v *visitor) varUsage(e ast.Expr) *varUsage {
	id, ok := e.(*ast.Ident)
	if !ok {
		return nil
	}
	param, ok := v.ObjectOf(id).(*types.Var)
	if !ok {
		return nil
	}
	if usage, e := v.vars[param]; e {
		return usage
	}
	usage := &varUsage{
		calls:    make(map[string]struct{}),
		assigned: make(map[*varUsage]struct{}),
	}
	v.vars[param] = usage
	return usage
}

func (v *visitor) addUsed(e ast.Expr, as types.Type) {
	if as == nil {
		return
	}
	usage := v.varUsage(e)
	if usage == nil {
		// not a variable
		return
	}
	iface, ok := as.Underlying().(*types.Interface)
	if !ok {
		usage.discard = true
		return
	}
	for i := 0; i < iface.NumMethods(); i++ {
		m := iface.Method(i)
		usage.calls[m.Name()] = struct{}{}
	}
}

func (v *visitor) addAssign(to, from ast.Expr) {
	pto := v.varUsage(to)
	pfrom := v.varUsage(from)
	if pto == nil || pfrom == nil {
		// either isn't a variable
		return
	}
	pfrom.assigned[pto] = struct{}{}
}

func (v *visitor) discard(e ast.Expr) {
	usage := v.varUsage(e)
	if usage == nil {
		// not a variable
		return
	}
	usage.discard = true
}

func (v *visitor) comparedWith(e, with ast.Expr) {
	if _, ok := with.(*ast.BasicLit); ok {
		v.discard(e)
	}
}

func (v *visitor) implementsIface(sign *types.Signature) bool {
	s := signString(sign)
	return v.funcOf(s) != ""
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
	var fd *funcDecl
	switch x := node.(type) {
	case *ast.FuncLit:
		fd = &funcDecl{
			sign:    v.Types[x].Type.(*types.Signature),
			astType: x.Type,
		}
		if v.implementsIface(fd.sign) {
			return nil
		}
	case *ast.FuncDecl:
		fd = &funcDecl{
			name:    x.Name.Name,
			sign:    v.Defs[x.Name].Type().(*types.Signature),
			astType: x.Type,
		}
		if v.implementsIface(fd.sign) {
			return nil
		}
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
		v.funcs = append(v.funcs, fd)
	}
	return v
}

func (v *visitor) onBinary(be *ast.BinaryExpr) {
	switch be.Op {
	case token.EQL, token.NEQ:
		v.comparedWith(be.X, be.Y)
		v.comparedWith(be.Y, be.X)
	default:
		v.discard(be.X)
		v.discard(be.Y)
	}
}

func (v *visitor) onAssign(as *ast.AssignStmt) {
	for i, val := range as.Rhs {
		left := as.Lhs[i]
		v.addUsed(val, v.Types[left].Type)
		v.addAssign(left, val)
	}
}

func compositeIdentType(t types.Type, i int) types.Type {
	switch x := t.(type) {
	case *types.Named:
		return compositeIdentType(x.Underlying(), i)
	case *types.Struct:
		return x.Field(i).Type()
	case *types.Array:
		return x.Elem()
	case *types.Slice:
		return x.Elem()
	default:
		return nil
	}
}

func (v *visitor) onComposite(cl *ast.CompositeLit) {
	for i, e := range cl.Elts {
		switch x := e.(type) {
		case *ast.KeyValueExpr:
			v.addUsed(x.Key, v.TypeOf(x.Value))
			v.addUsed(x.Value, v.TypeOf(x.Key))
		case *ast.Ident:
			v.addUsed(x, compositeIdentType(v.TypeOf(cl), i))
		}
	}
}

func (v *visitor) onCall(ce *ast.CallExpr) {
	switch x := v.TypeOf(ce.Fun).Underlying().(type) {
	case *types.Signature:
		v.onMethodCall(ce, x)
	default:
		// type conversion
		if len(ce.Args) == 1 {
			v.addUsed(ce.Args[0], x.Underlying())
		}
	}
}

func (v *visitor) onMethodCall(ce *ast.CallExpr, sign *types.Signature) {
	for i, e := range ce.Args {
		v.addUsed(e, paramType(sign, i))
	}
	sel, ok := ce.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	usage := v.varUsage(sel.X)
	if usage == nil {
		// not a variable
		return
	}
	usage.calls[sel.Sel.Name] = struct{}{}
}

type byPos []Warn

func (l byPos) Len() int { return len(l) }
func (l byPos) Less(i, j int) bool {
	p1, p2 := l[i].Pos, l[j].Pos
	if p1.Filename == p2.Filename {
		return p1.Offset < p2.Offset
	}
	return p1.Filename < p2.Filename
}
func (l byPos) Swap(i, j int) { l[i], l[j] = l[j], l[i] }

func (fd *funcDecl) paramGroups() [][]*types.Var {
	astList := fd.astType.Params.List
	groups := make([][]*types.Var, len(astList))
	signIndex := 0
	for i, field := range astList {
		group := make([]*types.Var, len(field.Names))
		for j := range field.Names {
			group[j] = fd.sign.Params().At(signIndex)
			signIndex++
		}
		groups[i] = group
	}
	return groups
}

func (v *visitor) funcEnded(fd *funcDecl) {
groupIter:
	for _, group := range fd.paramGroups() {
		var groupWarns []Warn
		for _, param := range group {
			usage := v.vars[param]
			if usage == nil {
				continue groupIter
			}
			newType := v.paramNewType(fd.name, param, usage)
			if newType == "" {
				continue groupIter
			}
			pos := v.fset.Position(param.Pos())
			// go/loader seems to like absolute paths
			if rel, err := filepath.Rel(v.wd, pos.Filename); err == nil {
				pos.Filename = rel
			}
			groupWarns = append(groupWarns, Warn{
				Pos:  pos,
				Name: param.Name(),
				Type: newType,
			})
		}
		v.warns = append(v.warns, groupWarns...)
	}
}

var fullPathParts = regexp.MustCompile(`^(\*)?(([^/]+/)*([^/]+)\.)?([^/]+)$`)

func (v *visitor) simpleName(fullName string) string {
	pname := v.Pkg.Path()
	if strings.HasPrefix(fullName, pname+".") {
		return fullName[len(pname)+1:]
	}
	m := fullPathParts.FindStringSubmatch(fullName)
	fullPkg := strings.TrimSuffix(m[2], ".")
	star, pkg, name := m[1], m[4], m[5]
	if name, e := v.impNames[fullPkg]; e {
		pkg = name
	}
	return star + pkg + "." + name
}

func isExported(name string) bool {
	ch, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(ch)
}

func willAddAllocation(t types.Type) bool {
	switch t.Underlying().(type) {
	case *types.Pointer, *types.Interface:
		return false
	default:
		return true
	}
}

func (v *visitor) paramNewType(funcName string, param *types.Var, usage *varUsage) string {
	t := param.Type()
	if !isExported(funcName) && willAddAllocation(t) {
		return ""
	}
	named := typeNamed(t)
	if named != nil {
		name := named.Obj().Name()
		if mentionsType(funcName, name) {
			return ""
		}
	}
	ifname, iftype := v.interfaceMatching(param, usage)
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
	return v.simpleName(ifname)
}

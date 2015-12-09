/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"log"
	"os"
)

func typeMap(t *types.Tuple) map[string]types.Type {
	m := make(map[string]types.Type, t.Len())
	for i := 0; i < t.Len(); i++ {
		p := t.At(i)
		m[p.Name()] = p.Type()
	}
	return m
}

func init() {
	if err := typesInit(); err != nil {
		log.Fatal(err)
	}
}

type call struct {
	params  []interface{}
	results []interface{}
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
	case string:
		return t1.String() == x
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
	case string:
		return t1.String() == x
	case nil:
		// assigning to _
		return true
	default:
		return false
	}
}

func resultsMatch(types1 []types.Type, exps2 []interface{}) bool {
	if len(exps2) == 0 {
		return true
	}
	if len(types1) != len(exps2) {
		return false
	}
	for i, t1 := range types1 {
		e2 := exps2[i]
		if !resultEqual(t1, e2) {
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

func main() {
	flag.Parse()
	if err := checkPaths(flag.Args(), os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func checkPaths(paths []string, w io.Writer) error {
	p := &goPkg{
		fset: token.NewFileSet(),
	}
	for _, fp := range paths {
		if err := p.parsePath(fp); err != nil {
			return err
		}
	}
	if err := p.check(w); err != nil {
		return err
	}
	return nil
}

type goPkg struct {
	fset  *token.FileSet
	files []*ast.File
}

func (p *goPkg) parsePath(fp string) error {
	f, err := os.Open(fp)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := p.parseReader(fp, f); err != nil {
		return err
	}
	return nil
}

func (p *goPkg) parseReader(name string, r io.Reader) error {
	f, err := parser.ParseFile(p.fset, name, r, 0)
	if err != nil {
		return err
	}
	p.files = append(p.files, f)
	return nil
}

func (p *goPkg) check(w io.Writer) error {
	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("", p.fset, p.files, nil)
	if err != nil {
		return err
	}

	v := &Visitor{
		w:      w,
		fset:   p.fset,
		scopes: []*types.Scope{pkg.Scope()},
	}
	for _, f := range p.files {
		ast.Walk(v, f)
	}
	return nil
}

type Visitor struct {
	w      io.Writer
	fset   *token.FileSet
	scopes []*types.Scope

	nodes []ast.Node

	params map[string]types.Type
	used   map[string]map[string]call
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

func (v *Visitor) recvFuncType(tname, fname string) *types.Func {
	_, obj := v.scope().LookupParent(tname, token.NoPos)
	st, ok := obj.(*types.TypeName)
	if !ok {
		return nil
	}
	named, ok := st.Type().(*types.Named)
	if !ok {
		return nil
	}
	for i := 0; i < named.NumMethods(); i++ {
		f := named.Method(i)
		if f.Name() == fname {
			return f
		}
	}
	return nil
}

func (v *Visitor) funcType(fd *ast.FuncDecl) *types.Func {
	fname := fd.Name.Name
	if fd.Recv == nil {
		f, ok := v.scope().Lookup(fname).(*types.Func)
		if !ok {
			return nil
		}
		return f
	}
	if len(fd.Recv.List) > 1 {
		return nil
	}
	tname := scopeName(fd.Recv.List[0].Type)
	return v.recvFuncType(tname, fname)
}

type assignCall struct {
	left  []ast.Expr
	right *ast.CallExpr
}

func assignCalls(as *ast.AssignStmt) []assignCall {
	if len(as.Rhs) == 1 {
		ce, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok {
			return nil
		}
		return []assignCall{
			{
				left:  as.Lhs,
				right: ce,
			},
		}
	}
	var calls []assignCall
	for i, right := range as.Rhs {
		ce, ok := right.(*ast.CallExpr)
		if !ok {
			continue
		}
		calls = append(calls, assignCall{
			left:  []ast.Expr{as.Lhs[i]},
			right: ce,
		})
	}
	return calls
}

func (v *Visitor) Visit(node ast.Node) ast.Visitor {
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
	case *ast.BlockStmt:
	case *ast.ExprStmt:
	case *ast.AssignStmt:
		calls := assignCalls(x)
		for _, c := range calls {
			v.onCall(c.left, c.right)
		}
		return nil
	case *ast.CallExpr:
		v.onCall(nil, x)
	case nil:
		top := v.nodes[len(v.nodes)-1]
		v.nodes = v.nodes[:len(v.nodes)-1]
		if _, ok := top.(*ast.FuncDecl); !ok {
			return nil
		}
		v.scopes = v.scopes[:len(v.scopes)-1]
		for name, methods := range v.used {
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
			pos := v.fset.Position(top.Pos())
			fmt.Fprintf(v.w, "%s:%d: %s can be %s\n",
				pos.Filename, pos.Line, name, iface)
		}
		v.params = nil
		v.used = nil
	default:
		return nil
	}
	if node != nil {
		v.nodes = append(v.nodes, node)
	}
	return v
}

func (v *Visitor) getType(name string) interface{} {
	_, obj := v.scope().LookupParent(name, token.NoPos)
	if obj == nil {
		return nil
	}
	switch x := obj.(type) {
	case *types.Var:
		return x.Type().String()
	default:
		return nil
	}
}

func (v *Visitor) descType(e ast.Expr) interface{} {
	switch x := e.(type) {
	case *ast.Ident:
		return v.getType(x.Name)
	case *ast.BasicLit:
		return x.Kind
	default:
		return nil
	}
}

func (v *Visitor) onCall(results []ast.Expr, ce *ast.CallExpr) {
	sel, ok := ce.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	left, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}
	right := sel.Sel
	vname := left.Name
	tname, _ := v.descType(left).(string)
	fname := right.Name
	c := call{}
	f := v.recvFuncType(tname, fname)
	if f != nil {
		sign := f.Type().(*types.Signature)
		results := sign.Results()
		for i := 0; i < results.Len(); i++ {
			v := results.At(i)
			c.results = append(c.results, v.Type().String())
		}
	} else {
		for _, r := range results {
			c.results = append(c.results, v.descType(r))
		}

	}
	for _, a := range ce.Args {
		c.params = append(c.params, v.descType(a))
	}
	if _, e := v.used[vname]; !e {
		v.used[vname] = make(map[string]call)
	}
	v.used[vname][fname] = c
}

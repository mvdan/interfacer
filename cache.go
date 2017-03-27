// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package interfacer

import (
	"go/ast"
	"go/types"
)

type pkgTypes struct {
	ifaces map[string]string
	funcs  map[string]string
}

func (p *pkgTypes) isFuncType(t string) bool {
	return p.funcs[t] != ""
}

func (p *pkgTypes) ifaceOf(t string) string {
	return p.ifaces[t]
}

func (p *pkgTypes) getTypes(pkg *types.Package) {
	p.ifaces = make(map[string]string)
	p.funcs = make(map[string]string)
	path := pkg.Path()
	addTypes := func(impPath string, ifs, funs map[string]string, top bool) {
		fullName := func(name string) string {
			if !top {
				return impPath + "." + name
			}
			return name
		}
		for iftype, name := range ifs {
			// only suggest exported interfaces
			if ast.IsExported(name) {
				p.ifaces[iftype] = fullName(name)
			}
		}
		for ftype, name := range funs {
			// ignore non-exported func signatures too
			p.funcs[ftype] = fullName(name)
		}
	}
	for _, imp := range pkg.Imports() {
		ifs, funs := fromScope(imp.Scope())
		addTypes(imp.Path(), ifs, funs, false)
	}
	ifs, funs := fromScope(pkg.Scope())
	addTypes(path, ifs, funs, true)
}

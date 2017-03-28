// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package interfacer

import (
	"go/ast"
	"go/types"
)

type pkgTypes struct {
	ifaces    map[string]string
	funcSigns map[string]bool
}

func (p *pkgTypes) getTypes(pkg *types.Package) {
	p.ifaces = make(map[string]string)
	p.funcSigns = make(map[string]bool)
	addTypes := func(impPath string, ifs map[string]string, funs map[string]bool, top bool) {
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
		for ftype := range funs {
			// ignore non-exported func signatures too
			p.funcSigns[ftype] = true
		}
	}
	for _, imp := range pkg.Imports() {
		ifs, funs := fromScope(imp.Scope())
		addTypes(imp.Path(), ifs, funs, false)
	}
	ifs, funs := fromScope(pkg.Scope())
	addTypes(pkg.Path(), ifs, funs, true)
}

// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package interfacer

import (
	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/types"
)

//go:generate sh -c "go list std | go run generate/std/main.go -o std.go"
//go:generate gofmt -w -s std.go

type cache struct {
	loader.Config

	grabbed map[string]struct{}
}

func typesInit() {
	c = &cache{
		grabbed: make(map[string]struct{}),
	}
	c.AllowErrors = true
	c.TypeChecker.Error = func(e error) {}
	c.TypeChecker.DisableUnusedImportCheck = true
	c.TypeCheckFuncBodies = func(path string) bool {
		if _, e := stdPkgs[path]; e {
			return false
		}
		return true
	}
}

func typesGet(pkgs []*types.Package) {
	for _, pkg := range pkgs {
		path := pkg.Path()
		if _, e := stdPkgs[path]; e {
			continue
		}
		if _, e := c.grabbed[path]; e {
			continue
		}
		c.grabbed[path] = struct{}{}
		grabExported(pkg.Scope(), path)
		typesGet(pkg.Imports())
	}
}

func grabExported(scope *types.Scope, path string) {
	ifs, funs := FromScope(scope, false)
	for iftype, ifname := range ifs {
		if _, e := ifaces[iftype]; e {
			continue
		}
		ifaces[iftype] = path + "." + ifname
	}
	for ftype, fname := range funs {
		if _, e := funcs[ftype]; e {
			continue
		}
		funcs[ftype] = path + "." + fname
	}
}

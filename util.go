// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package interfacer

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/tools/go/types"
)

const (
	maxLenFunc  = 160
	maxLenIface = 320
)

type ByAlph []string

func (l ByAlph) Len() int           { return len(l) }
func (l ByAlph) Less(i, j int) bool { return l[i] < l[j] }
func (l ByAlph) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

var exported = regexp.MustCompile(`^[A-Z]`)

func ifaceFuncMap(iface *types.Interface) map[string]string {
	ifuncs := make(map[string]string, iface.NumMethods())
	for i := 0; i < iface.NumMethods(); i++ {
		f := iface.Method(i)
		fname := f.Name()
		if !exported.MatchString(fname) {
			return nil
		}
		sign := f.Type().(*types.Signature)
		ifuncs[fname] = signString(sign)
	}
	return ifuncs
}

func namedMethodMap(named *types.Named) map[string]string {
	ifuncs := make(map[string]string)
	for i := 0; i < named.NumMethods(); i++ {
		f := named.Method(i)
		fname := f.Name()
		if !exported.MatchString(fname) {
			continue
		}
		sign := f.Type().(*types.Signature)
		ifuncs[fname] = signString(sign)
	}
	return ifuncs
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

func funcMapString(iface map[string]string) string {
	fnames := make([]string, 0, len(iface))
	for fname := range iface {
		fnames = append(fnames, fname)
	}
	sort.Sort(ByAlph(fnames))
	var b bytes.Buffer
	for i, fname := range fnames {
		if i > 0 {
			io.WriteString(&b, "; ")
		}
		io.WriteString(&b, fname)
		io.WriteString(&b, iface[fname])
	}
	return b.String()
}

func tupleStrs(t *types.Tuple) []string {
	l := make([]string, 0, t.Len())
	for i := 0; i < t.Len(); i++ {
		v := t.At(i)
		l = append(l, v.Type().String())
	}
	return l
}

func signString(sign *types.Signature) string {
	ps := tupleStrs(sign.Params())
	rs := tupleStrs(sign.Results())
	if len(rs) == 0 {
		return fmt.Sprintf("(%s)", strings.Join(ps, ", "))
	}
	if len(rs) == 1 {
		return fmt.Sprintf("(%s) %s", strings.Join(ps, ", "), rs[0])
	}
	return fmt.Sprintf("(%s) (%s)", strings.Join(ps, ", "), strings.Join(rs, ", "))
}

func interesting(t types.Type) bool {
	switch x := t.(type) {
	case *types.Interface:
		return x.NumMethods() > 1
	case *types.Struct:
		return true
	case *types.Named:
		return interesting(x.Underlying())
	case *types.Pointer:
		return interesting(x.Elem())
	default:
		return false
	}
}

func anyInteresting(params *types.Tuple) bool {
	for i := 0; i < params.Len(); i++ {
		t := params.At(i).Type()
		if interesting(t) {
			return true
		}
	}
	return false
}

func FromScope(scope *types.Scope, all bool) (map[string]string, map[string]string) {
	ifaces := make(map[string]string)
	funcs := make(map[string]string)
	signStr := func(sign *types.Signature) string {
		if !anyInteresting(sign.Params()) {
			return ""
		}
		s := signString(sign)
		if len(s) > maxLenFunc {
			return ""
		}
		return s
	}
	for _, name := range scope.Names() {
		if !all && !exported.MatchString(name) {
			continue
		}
		tn, ok := scope.Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		switch x := tn.Type().Underlying().(type) {
		case *types.Interface:
			iface := ifaceFuncMap(x)
			if len(iface) == 0 {
				continue
			}
			for i := 0; i < x.NumMethods(); i++ {
				f := x.Method(i)
				if _, e := iface[f.Name()]; !e {
					continue
				}
				sign := f.Type().(*types.Signature)
				if s := signStr(sign); s != "" {
					funcs[s] = tn.Name() + "." + f.Name()
				}
			}
			s := funcMapString(iface)
			if len(s) > maxLenIface {
				continue
			}
			ifaces[s] = tn.Name()
		case *types.Signature:
			if s := signStr(x); s != "" {
				funcs[s] = tn.Name()
			}
		}
	}
	return ifaces, funcs
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

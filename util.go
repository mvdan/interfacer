// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package interfacer

import (
	"bytes"
	"fmt"
	"go/types"
	"io"
	"regexp"
	"sort"
	"strings"
)

type ByAlph []string

func (l ByAlph) Len() int           { return len(l) }
func (l ByAlph) Less(i, j int) bool { return l[i] < l[j] }
func (l ByAlph) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

var exported = regexp.MustCompile(`^[A-Z]`)

type methoder interface {
	NumMethods() int
	Method(int) *types.Func
}

func methoderFuncMap(m methoder, skip bool) map[string]string {
	ifuncs := make(map[string]string, m.NumMethods())
	for i := 0; i < m.NumMethods(); i++ {
		f := m.Method(i)
		fname := f.Name()
		if !exported.MatchString(fname) {
			if skip {
				continue
			}
			return nil
		}
		sign := f.Type().(*types.Signature)
		ifuncs[fname] = signString(sign)
	}
	return ifuncs
}

func typeFuncMap(t types.Type) map[string]string {
	switch x := t.(type) {
	case *types.Pointer:
		return typeFuncMap(x.Elem())
	case *types.Named:
		if u, ok := x.Underlying().(*types.Interface); ok {
			return typeFuncMap(u)
		}
		return methoderFuncMap(x, true)
	case *types.Interface:
		return methoderFuncMap(x, false)
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

func FromScope(scope *types.Scope, all bool) (ifaces, funcs map[string]string) {
	ifaces = make(map[string]string)
	funcs = make(map[string]string)
	ifaceFuncs := make(map[string]string)
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
			iface := methoderFuncMap(x, false)
			if len(iface) == 0 {
				continue
			}
			for i := 0; i < x.NumMethods(); i++ {
				f := x.Method(i)
				sign := f.Type().(*types.Signature)
				if !anyInteresting(sign.Params()) {
					continue
				}
				s := signString(sign)
				if _, e := ifaceFuncs[s]; e {
					continue
				}
				ifaceFuncs[s] = tn.Name() + "." + f.Name()
			}
			s := funcMapString(iface)
			if _, e := ifaces[s]; !e {
				ifaces[s] = tn.Name()
			}
		case *types.Signature:
			if !anyInteresting(x.Params()) {
				continue
			}
			s := signString(x)
			if _, e := funcs[s]; !e {
				funcs[s] = tn.Name()
			}
		}
	}
	for s, name := range ifaceFuncs {
		if _, e := funcs[s]; !e {
			funcs[s] = name
		}
	}
	return ifaces, funcs
}

func mentionsType(fname, tname string) bool {
	if fname == "" || tname == "" {
		return false
	}
	upper := strings.ToUpper(tname)
	capit := strings.ToUpper(tname[:1]) + tname[1:]
	lower := strings.ToLower(tname)
	uncap := strings.ToLower(tname[:1]) + tname[1:]
	upperNames := fmt.Sprintf(`(%s|%s)`, upper, capit)
	allNames := fmt.Sprintf(`(%s|%s|%s)`, upperNames, lower, uncap)
	exp := fmt.Sprintf(`^%s[A-Z]|%s([A-Z]|$)`, allNames, upperNames)
	match, err := regexp.MatchString(exp, fname)
	if err != nil {
		panic(err)
	}
	return match
}

func typeNamed(t types.Type) *types.Named {
	for {
		switch x := t.(type) {
		case *types.Named:
			return x
		case *types.Pointer:
			t = x.Elem()
		case *types.Array:
			t = x.Elem()
		case *types.Slice:
			t = x.Elem()
		default:
			return nil
		}
	}
}

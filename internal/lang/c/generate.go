package c

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

func ctype(t string) string {
	b, d := problem.SplitArray(t)
	v := map[string]string{"integer": "int", "long": "long long", "double": "double", "boolean": "bool", "character": "char", "string": "char*", "void": "void", "ListNode": "struct ListNode*", "TreeNode": "struct TreeNode*", "NestedInteger": "struct NestedInteger*", "Employee": "struct Employee*"}[b]
	if problem.IsNodeVariant(b) {
		v = "struct Node*"
	}
	return v + strings.Repeat("*", d)
}
func code(t string) string {
	b, _ := problem.SplitArray(t)
	if problem.IsNodeVariant(b) {
		return "HC_NODE"
	}
	return "HC_" + strings.ToUpper(b)
}
func pointer(t string) bool {
	b, d := problem.SplitArray(t)
	return d > 0 || b == "string" || b == "NestedInteger" || b == "Employee" || problem.IsNodeType(b)
}
func value(n, t string) string {
	if pointer(t) {
		return "(" + ctype(t) + ")" + n + ".data"
	}
	return "*(" + ctype(t) + "*)" + n + ".data"
}
func arguments(n, t string) []string {
	a := []string{value(n, t)}
	_, d := problem.SplitArray(t)
	if d > 0 {
		a = append(a, n+".size")
	}
	if d > 1 {
		a = append(a, n+".rows")
	}
	if d > 2 {
		a = append(a, n+".cols")
	}
	return a
}
func decls(ps []problem.Param) []string {
	var a []string
	for i, p := range ps {
		n := "arg" + strconv.Itoa(i)
		a = append(a, ctype(p.Type)+" "+n)
		_, d := problem.SplitArray(p.Type)
		if d > 0 {
			a = append(a, "int "+n+"Size")
		}
		if d > 1 {
			a = append(a, "int* "+n+"ColSize")
		}
		if d > 2 {
			a = append(a, "int** "+n+"DepthSizes")
		}
	}
	return a
}
func returnDecls(t string) []string {
	_, d := problem.SplitArray(t)
	var a []string
	if d > 0 {
		a = append(a, "int* returnSize")
	}
	if d > 1 {
		a = append(a, "int** returnColumnSizes")
	}
	if d > 2 {
		a = append(a, "int*** returnDepthSizes")
	}
	return a
}
func binding(p *problem.Problem) problem.CBinding {
	b := problem.CBinding{}
	if p.C != nil {
		b = *p.C
	}
	if p.Kind == problem.KindFunction {
		if b.Function == "" {
			b.Function = p.Function.Name
		}
		return b
	}
	if b.Object == "" {
		b.Object = "typedef"
	}
	if b.Type == "" {
		b.Type = p.Class.Name
	}
	prefix := strings.ToLower(p.Class.Name[:1]) + p.Class.Name[1:]
	if b.Constructor == "" {
		b.Constructor = prefix + "Create"
	}
	if b.Destructor == nil {
		s := prefix + "Free"
		b.Destructor = &s
	}
	methods := map[string]string{}
	for _, m := range p.Class.Methods {
		if b.Object == "none" {
			methods[m.Name] = m.Name
		} else {
			methods[m.Name] = prefix + strings.ToUpper(m.Name[:1]) + m.Name[1:]
		}
	}
	for k, v := range b.Methods {
		methods[k] = v
	}
	b.Methods = methods
	return b
}
func objectType(b problem.CBinding) string {
	if b.Object == "struct" {
		return "struct " + b.Type + "*"
	}
	return b.Type + "*"
}
func (Language) Starter(p *problem.Problem) (string, error) {
	if b, e := os.ReadFile(filepath.Join(p.Dir, "templates", "solution.c")); e == nil {
		return string(b), nil
	} else if !os.IsNotExist(e) {
		return "", e
	}
	b := binding(p)
	var s strings.Builder
	s.WriteString("/* Standard headers, uthash and judge types are provided by hc. */\n")
	stub := func(name, ret string, params []string) {
		if len(params) == 0 {
			params = []string{"void"}
		}
		fmt.Fprintf(&s, "%s %s(%s) {\n    abort(); /* not implemented */\n}\n", ret, name, strings.Join(params, ", "))
	}
	if p.Kind == problem.KindFunction {
		stub(b.Function, ctype(p.Function.Returns), append(decls(p.Function.Params), returnDecls(p.Function.Returns)...))
	} else {
		if b.Object != "none" {
			if b.Object == "struct" {
				fmt.Fprintf(&s, "struct %s { int placeholder; };\n", b.Type)
			} else {
				fmt.Fprintf(&s, "typedef struct { int placeholder; } %s;\n", b.Type)
			}
			stub(b.Constructor, objectType(b), decls(p.Class.Constructor))
		}
		for _, m := range p.Class.Methods {
			a := decls(m.Params)
			if b.Object != "none" {
				a = append([]string{objectType(b) + " obj"}, a...)
			}
			stub(b.Methods[m.Name], ctype(m.Returns), append(a, returnDecls(m.Returns)...))
		}
		if b.Object != "none" && *b.Destructor != "" {
			fmt.Fprintf(&s, "void %s(%s obj) { free(obj); }\n", *b.Destructor, objectType(b))
		}
	}
	return s.String(), nil
}
func decode(s *strings.Builder, n, t, expr string) {
	_, d := problem.SplitArray(t)
	fmt.Fprintf(s, "hc_value %s=hc_decode(%s,%s,%d);\n", n, expr, code(t), d)
}
func encode(n, t string) string {
	_, d := problem.SplitArray(t)
	return fmt.Sprintf("hc_encode(%s,%s,%d)", n, code(t), d)
}
func emitArgs(s *strings.Builder, ps []problem.Param, from, prefix string) []string {
	var args []string
	for i, p := range ps {
		n := prefix + strconv.Itoa(i)
		decode(s, n, p.Type, fmt.Sprintf("yyjson_arr_get(%s,%d)", from, i))
		args = append(args, arguments(n, p.Type)...)
	}
	return args
}
func call(s *strings.Builder, name, ret string, args []string) {
	_, d := problem.SplitArray(ret)
	s.WriteString("hc_value result={0};\n")
	if d > 0 {
		args = append(args, "&result.size")
	}
	if d > 1 {
		args = append(args, "&result.rows")
	}
	if d > 2 {
		args = append(args, "&result.cols")
	}
	s.WriteString("hc_start();\n")
	expr := name + "(" + strings.Join(args, ",") + ")"
	if ret == "void" {
		fmt.Fprintf(s, "%s;\n", expr)
	} else {
		fmt.Fprintf(s, "%s returned=%s;\n", ctype(ret), expr)
	}
	s.WriteString("hc_stop();\n")
	if ret != "void" {
		if pointer(ret) {
			s.WriteString("result.data=returned;\n")
		} else {
			s.WriteString("result.data=&returned;\n")
		}
	}
}
func Driver(p *problem.Problem) (string, error) {
	if err := p.ValidateC(); err != nil {
		return "", err
	}
	b := binding(p)
	var s strings.Builder
	s.WriteString("#define _POSIX_C_SOURCE 200809L\n#include \"hc_c_types.h\"\n#include \"hc_c_runtime.h\"\n#include \"solution.c\"\nstatic yyjson_mut_val *hc_call(yyjson_val *input) {\n")
	if enc, dec, ok := p.RoundTripMethods(); ok {
		decode(&s, "arg", enc.Params[0].Type, fmt.Sprintf("yyjson_obj_get(input,%q)", enc.Params[0].Name))
		if b.Object != "none" {
			fmt.Fprintf(&s, "hc_start(); %s obj=%s(); hc_stop();\n", objectType(b), b.Constructor)
		}
		a := arguments("arg", enc.Params[0].Type)
		if b.Object != "none" {
			a = append([]string{"obj"}, a...)
		}
		fmt.Fprintf(&s, "hc_start(); char *text=%s(%s); hc_stop();\n", b.Methods[enc.Name], strings.Join(a, ","))
		if b.Object != "none" && p.Judge.RoundTrip.Instances == problem.InstancesSeparate {
			fmt.Fprintf(&s, "hc_start(); %s second=%s(); hc_stop();\n", objectType(b), b.Constructor)
		}
		a = []string{"text"}
		if b.Object != "none" {
			obj := "obj"
			if p.Judge.RoundTrip.Instances == problem.InstancesSeparate {
				obj = "second"
			}
			a = append([]string{obj}, a...)
		}
		call(&s, b.Methods[dec.Name], dec.Returns, a)
		fmt.Fprintf(&s, "yyjson_mut_val *out=%s;\n", encode("result", dec.Returns))
		if b.Object != "none" && *b.Destructor != "" {
			fmt.Fprintf(&s, "%s(obj);\n", *b.Destructor)
			if p.Judge.RoundTrip.Instances == problem.InstancesSeparate {
				fmt.Fprintf(&s, "%s(second);\n", *b.Destructor)
			}
		}
		s.WriteString("return out;\n")
	} else if p.Kind == problem.KindClass {
		s.WriteString("yyjson_val *ops=yyjson_obj_get(input,\"ops\"), *args=yyjson_obj_get(input,\"args\"); yyjson_mut_val *out=hc_arr();\n")
		a := emitArgs(&s, p.Class.Constructor, "yyjson_arr_get(args,0)", "ctor")
		if b.Object != "none" {
			fmt.Fprintf(&s, "hc_start(); %s obj=%s(%s); hc_stop();\n", objectType(b), b.Constructor, strings.Join(a, ","))
		}
		s.WriteString("hc_add(out,hc_null()); for(size_t i=1;i<yyjson_arr_size(ops);i++){yyjson_val *a=yyjson_arr_get(args,i); const char *op=yyjson_get_str(yyjson_arr_get(ops,i));\n")
		for i, m := range p.Class.Methods {
			prefix := ""
			if i > 0 {
				prefix = "else "
			}
			fmt.Fprintf(&s, "%sif(!strcmp(op,%q)){\n", prefix, m.Name)
			a = emitArgs(&s, m.Params, "a", "p")
			if b.Object != "none" {
				a = append([]string{"obj"}, a...)
			}
			call(&s, b.Methods[m.Name], m.Returns, a)
			fmt.Fprintf(&s, "hc_add(out,%s);}\n", encode("result", m.Returns))
		}
		s.WriteString("else hc_fail(\"unknown method\");}\n")
		if b.Object != "none" && *b.Destructor != "" {
			fmt.Fprintf(&s, "%s(obj);\n", *b.Destructor)
		}
		s.WriteString("return out;\n")
	} else {
		f := p.Function
		names := map[string]string{}
		all := append(append([]problem.Param{}, f.Params...), f.Hidden...)
		for i, p := range all {
			names[p.Name] = fmt.Sprintf("p%d", i)
		}
		for _, p := range all {
			if p.Copy != "" || p.Ref != "" {
				continue
			}
			n := names[p.Name]
			decode(&s, n, p.Type, fmt.Sprintf("yyjson_obj_get(input,%q)", p.Name))
			if p.Type == "ListNode" || p.Type == "TreeNode" {
				fmt.Fprintf(&s, "hc_nodes %s_nodes=hc_collect(%s.data,%s);\n", n, n, code(p.Type))
			}
		}
		for _, p := range f.Params {
			n := names[p.Name]
			if p.Cycle != "" {
				fmt.Fprintf(&s, "hc_cycle(%s_nodes,%s);\n", n, value(names[p.Cycle], "integer"))
			}
			if p.Join != nil {
				j := p.Join
				fmt.Fprintf(&s, "%s=hc_join(%s,%s,%s,%s); %s_nodes=hc_collect(%s.data,HC_LISTNODE);\n", n, n, names[j.List], value(names[j.Skip], "integer"), value(names[j.At], "integer"), n, n)
			}
		}
		for _, p := range f.Params {
			n := names[p.Name]
			if p.Copy != "" {
				fmt.Fprintf(&s, "hc_value %s=hc_clone(%s,%s); hc_nodes %s_nodes=hc_collect(%s.data,%s);\n", n, names[p.Copy], code(p.Type), n, n, code(p.Type))
			}
		}
		for _, p := range f.Params {
			if p.Ref != "" {
				fmt.Fprintf(&s, "hc_value %s={.data=hc_ref(%s_nodes,yyjson_get_int(yyjson_obj_get(input,%q)))};\n", names[p.Name], names[p.Ref], p.Name)
			}
		}
		if f.ReturnsCopy != "" {
			fmt.Fprintf(&s, "hc_nodes before=hc_collect(%s.data,%s);\n", names[f.ReturnsCopy], code(f.Returns))
		}
		var a []string
		for _, p := range f.Params {
			a = append(a, arguments(names[p.Name], p.Type)...)
		}
		call(&s, b.Function, f.Returns, a)
		if f.ReturnsCopy != "" {
			fmt.Fprintf(&s, "hc_check_copy(result.data,%s,before);\n", code(f.Returns))
		}
		switch {
		case p.Judge.Output != "":
			n := names[p.Judge.Output]
			prm, _ := f.Input(p.Judge.Output)
			if p.Judge.OutputLength != "" {
				fmt.Fprintf(&s, "hc_check(returned>=0 && returned<=%s.size,\"returned length is outside the output array\"); %s.size=(int)returned;\n", n, n)
			}
			fmt.Fprintf(&s, "return %s;\n", encode(n, prm.Type))
		case f.ReturnsRef != "":
			fmt.Fprintf(&s, "return hc_identity(result.data,%s_nodes,%s);\n", names[f.ReturnsRef], code(f.Returns))
		default:
			fmt.Fprintf(&s, "return %s;\n", encode("result", f.Returns))
		}
	}
	s.WriteString("}\nint main(void){return hc_run();}\n")
	return s.String(), nil
}

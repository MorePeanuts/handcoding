package cpp

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

func cppType(t string) string {
	base, depth := problem.SplitArray(t)
	x := map[string]string{"integer": "int", "long": "long long", "double": "double", "boolean": "bool", "character": "char", "string": "string", "void": "void", "ListNode": "ListNode*", "TreeNode": "TreeNode*", "NestedInteger": "NestedInteger", "Employee": "Employee*"}[base]
	if problem.IsNodeVariant(base) {
		x = "Node*"
	}
	for range depth {
		x = "vector<" + x + ">"
	}
	return x
}
func signature(ps []problem.Param) string {
	parts := make([]string, len(ps))
	for i, p := range ps {
		typ := cppType(p.Type)
		if p.Type == "string" || strings.HasSuffix(p.Type, "[]") {
			typ += "&"
		}
		parts[i] = typ + " arg" + strconv.Itoa(i)
	}
	return strings.Join(parts, ", ")
}
func (Language) Starter(p *problem.Problem) (string, error) {
	b, err := os.ReadFile(filepath.Join(p.Dir, "templates", "solution.cpp"))
	if err == nil {
		return string(b), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	var s strings.Builder
	s.WriteString("// Standard headers and judge-defined types are provided by hc.\n")
	if p.Kind == problem.KindFunction {
		fmt.Fprintf(&s, "class Solution {\npublic:\n    %s %s(%s) {\n        throw logic_error(\"not implemented\");\n    }\n};\n", cppType(p.Function.Returns), p.Function.Name, signature(p.Function.Params))
	} else {
		fmt.Fprintf(&s, "class %s {\npublic:\n    %s(%s) {}\n", p.Class.Name, p.Class.Name, signature(p.Class.Constructor))
		for _, m := range p.Class.Methods {
			fmt.Fprintf(&s, "    %s %s(%s) { throw logic_error(\"not implemented\"); }\n", cppType(m.Returns), m.Name, signature(m.Params))
		}
		s.WriteString("};\n")
	}
	return s.String(), nil
}
func Driver(p *problem.Problem) (string, error) {
	for _, t := range p.BaseTypes() {
		if cppType(t) == "" {
			return "", fmt.Errorf("cpp: unsupported type %s", t)
		}
	}
	var s strings.Builder
	s.WriteString("#include \"hc_types.hpp\"\n#include \"hc_runtime.hpp\"\n#include \"solution.cpp\"\nnamespace hc {\nJ call(const J& input) {\n")
	if enc, dec, ok := p.RoundTripMethods(); ok {
		fmt.Fprintf(&s, "auto arg=decode<%s>(input.at(%q));\n", cppType(enc.Params[0].Type), enc.Params[0].Name)
		fmt.Fprintf(&s, "auto first=timed([&]{return make_unique<::%s>();});\nauto text=timed([&]{return first->%s(arg);});\n", p.Class.Name, enc.Name)
		obj := "first"
		if p.Judge.RoundTrip.Instances == problem.InstancesSeparate {
			fmt.Fprintf(&s, "auto second=timed([&]{return make_unique<::%s>();});\n", p.Class.Name)
			obj = "second"
		}
		fmt.Fprintf(&s, "auto value=timed([&]{return %s->%s(text);});\nreturn encode(value);\n", obj, dec.Name)
	} else if p.Kind == problem.KindClass {
		s.WriteString("const auto& ops=input.at(\"ops\"); const auto& args=input.at(\"args\");\nJ output=J::array();\n")
		names := emitArgs(&s, p.Class.Constructor, "args.at(0)", "ctor")
		fmt.Fprintf(&s, "auto obj=timed([&]{return make_unique<::%s>(%s);});\noutput.push_back(nullptr);\nfor(size_t i=1;i<ops.size();++i){\nconst auto& a=args.at(i);\n", p.Class.Name, strings.Join(names, ","))
		for i, m := range p.Class.Methods {
			fmt.Fprintf(&s, "%sif(ops[i]==%q){\n", map[bool]string{true: "", false: "else "}[i == 0], m.Name)
			names = emitArgs(&s, m.Params, "a", "arg")
			emitCall(&s, "obj->"+m.Name+"("+strings.Join(names, ",")+")", m.Returns, "result")
			if m.Returns == problem.TypeVoid {
				s.WriteString("output.push_back(nullptr);\n")
			} else {
				s.WriteString("output.push_back(encode(result));\n")
			}
			s.WriteString("}\n")
		}
		s.WriteString("else {throw runtime_error(\"unknown method\");}\n}\nreturn output;\n")
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
			fmt.Fprintf(&s, "auto %s=decode<%s>(input.at(%q));\n", n, cppType(p.Type), p.Name)
			if p.Type == problem.TypeListNode || p.Type == problem.TypeTreeNode {
				fmt.Fprintf(&s, "auto %s_nodes=nodes(%s);\n", n, n)
			}
		}
		for _, p := range f.Params {
			n := names[p.Name]
			if p.Cycle != "" {
				pos := names[p.Cycle]
				fmt.Fprintf(&s, "if(%s>=0) %s_nodes.back()->next=%s_nodes.at(%s);\n", pos, n, n, pos)
			}
			if p.Join != nil {
				j := p.Join
				a, skip, at := names[j.List], names[j.Skip], names[j.At]
				fmt.Fprintf(&s, "%s_nodes.resize(%s);\n%s_nodes.insert(%s_nodes.end(),%s_nodes.begin()+%s,%s_nodes.end());\nif(%s>0) %s_nodes.at(%s-1)->next=(static_cast<size_t>(%s)<%s_nodes.size()?%s_nodes[%s]:nullptr);\n%s=%s_nodes.empty()?nullptr:%s_nodes[0];\n", n, skip, n, n, a, at, a, skip, n, skip, skip, n, n, skip, n, n, n)
			}
		}
		for _, p := range f.Params {
			n := names[p.Name]
			if p.Copy != "" {
				fmt.Fprintf(&s, "auto %s=decode<%s>(encode(%s));auto %s_nodes=nodes(%s);\n", n, cppType(p.Type), names[p.Copy], n, n)
			}
		}
		for _, p := range f.Params {
			if p.Ref != "" {
				fmt.Fprintf(&s, "auto %s=ref(%s_nodes,input.at(%q).get<int>());\n", names[p.Name], names[p.Ref], p.Name)
			}
		}
		if f.ReturnsCopy != "" {
			fmt.Fprintf(&s, "auto before=node_set(%s);\n", names[f.ReturnsCopy])
		}
		s.WriteString("Solution solution;\n")
		args := []string{}
		for _, p := range f.Params {
			args = append(args, names[p.Name])
		}
		emitCall(&s, "solution."+f.Name+"("+strings.Join(args, ",")+")", f.Returns, "result")
		if f.ReturnsCopy != "" {
			s.WriteString("check_copy(result,before);\n")
		}
		switch {
		case p.Judge.Output != "":
			n := names[p.Judge.Output]
			if p.Judge.OutputLength != "" {
				fmt.Fprintf(&s, "check(result>=0&&static_cast<unsigned long long>(result)<=%s.size(),\"returned length is outside the output array\");%s.resize(static_cast<size_t>(result));\n", n, n)
			}
			fmt.Fprintf(&s, "return encode(%s);\n", n)
		case f.ReturnsRef != "":
			fmt.Fprintf(&s, "return identity(result,%s_nodes);\n", names[f.ReturnsRef])
		case f.Returns == problem.TypeVoid:
			s.WriteString("return nullptr;\n")
		default:
			s.WriteString("return encode(result);\n")
		}
	}
	s.WriteString("}\n}\n")
	return s.String(), nil
}
func emitArgs(s *strings.Builder, ps []problem.Param, from, prefix string) []string {
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = prefix + strconv.Itoa(i)
		fmt.Fprintf(s, "auto %s=decode<%s>(%s.at(%d));\n", names[i], cppType(p.Type), from, i)
	}
	return names
}
func emitCall(s *strings.Builder, expr, typ, name string) {
	if typ == problem.TypeVoid {
		fmt.Fprintf(s, "timed([&]{%s;});\n", expr)
	} else {
		fmt.Fprintf(s, "auto %s=timed([&]{return %s;});\n", name, expr)
	}
}

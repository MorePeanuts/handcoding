package problem

import (
	"fmt"
	"regexp"
)

// CBinding describes C symbols, not JSON types. Object is typedef (default),
// struct, or none for stateless functions such as LeetCode's tree codecs.
type CBinding struct {
	Function    string            `yaml:"function,omitempty"`
	Object      string            `yaml:"object,omitempty"`
	Type        string            `yaml:"type,omitempty"`
	Constructor string            `yaml:"constructor,omitempty"`
	Destructor  *string           `yaml:"destructor,omitempty"`
	Methods     map[string]string `yaml:"methods,omitempty"`
	Libraries   []string          `yaml:"libraries,omitempty"`
}

var cIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func CIdentifier(s string) bool { return cIdentifier.MatchString(s) }
func (p *Problem) ValidateC() error {
	b := p.C
	if b == nil {
		return nil
	}
	for _, s := range []string{b.Function, b.Type, b.Constructor} {
		if s != "" && !CIdentifier(s) {
			return fmt.Errorf("c binding: invalid C identifier %q", s)
		}
	}
	if b.Destructor != nil && *b.Destructor != "" && !CIdentifier(*b.Destructor) {
		return fmt.Errorf("c.destructor: invalid C identifier")
	}
	if b.Object != "" && b.Object != "typedef" && b.Object != "struct" && b.Object != "none" {
		return fmt.Errorf("c.object must be typedef, struct or none")
	}
	if p.Kind == KindFunction && (b.Object != "" || b.Type != "" || b.Constructor != "" || b.Destructor != nil || len(b.Methods) > 0) {
		return fmt.Errorf("c: object bindings require a class problem")
	}
	if p.Kind == KindClass && b.Function != "" {
		return fmt.Errorf("c.function requires a function problem")
	}
	if b.Object == "none" && (b.Type != "" || b.Constructor != "" || b.Destructor != nil || p.Class != nil && len(p.Class.Constructor) > 0) {
		return fmt.Errorf("c.object none cannot have an object type, constructor, destructor or constructor parameters")
	}
	for name, symbol := range b.Methods {
		found := false
		if p.Class != nil {
			for _, m := range p.Class.Methods {
				if m.Name == name {
					found = true
				}
			}
		}
		if !found || !CIdentifier(symbol) {
			return fmt.Errorf("invalid C method binding %q: %q", name, symbol)
		}
	}
	for _, library := range b.Libraries {
		if library != "glib" {
			return fmt.Errorf("unknown C library %q (supported: glib)", library)
		}
	}
	return nil
}

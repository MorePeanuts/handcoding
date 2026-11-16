package c

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/MorePeanuts/handcoding/internal/lang"
)

func (l Language) pkgConfig() string {
	if l.PkgConfig != "" {
		return l.PkgConfig
	}
	return "pkg-config"
}
func (l Language) glibFlags(ctx context.Context, required bool) ([]string, []string, error) {
	mode := l.GLib
	if mode == "" {
		mode = "auto"
	}
	if mode != "auto" && mode != "required" && mode != "disabled" {
		return nil, nil, fmt.Errorf("c.glib must be auto, required or disabled")
	}
	if mode == "disabled" {
		if required {
			return nil, nil, fmt.Errorf("this problem requires GLib but c.glib is disabled")
		}
		return nil, nil, nil
	}
	required = required || mode == "required"
	probe := func() (flags, libs []string, err error) {
		p, e := lang.ResolveExecutable(l.pkgConfig())
		if e != nil {
			return nil, nil, e
		}
		f, e := lang.Probe(ctx, p, nil, "--cflags", "glib-2.0")
		if e != nil {
			return nil, nil, e
		}
		b, e := lang.Probe(ctx, p, nil, "--libs", "glib-2.0")
		if e != nil {
			return nil, nil, e
		}
		flags, e = splitFlags(string(f))
		if e != nil {
			return nil, nil, e
		}
		libs, e = splitFlags(string(b))
		return flags, libs, e
	}
	flags, libs, err := probe()
	if err != nil {
		if required {
			return nil, nil, fmt.Errorf("GLib development files and pkg-config are required: install them locally and run hc doctor -l c: %w", err)
		}
		return nil, nil, nil
	}
	return append(flags, "-DHC_GLIB=1"), libs, nil
}

// pkg-config quotes spaces with shell quoting; parse argv without executing a shell.
func splitFlags(s string) ([]string, error) {
	var args []string
	var token strings.Builder
	var quote rune
	escape := false
	active := false
	for _, r := range s {
		if escape {
			token.WriteRune(r)
			escape = false
			active = true
			continue
		}
		if r == '\\' && quote != '\'' {
			escape = true
			active = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				token.WriteRune(r)
			}
			active = true
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			active = true
			continue
		}
		if unicode.IsSpace(r) {
			if active {
				args = append(args, token.String())
				token.Reset()
				active = false
			}
			continue
		}
		token.WriteRune(r)
		active = true
	}
	if quote != 0 || escape {
		return nil, fmt.Errorf("malformed pkg-config flags")
	}
	if active {
		args = append(args, token.String())
	}
	return args, nil
}

package perm

import (
	"strings"
	"testing"
)

func TestDefsConsistent(t *testing.T) {
	codes := map[string]Def{}
	for _, d := range Defs {
		if _, dup := codes[d.Code]; dup {
			t.Fatalf("duplicate code %q", d.Code)
		}
		codes[d.Code] = d
	}
	for _, d := range Defs {
		if d.Parent != "" {
			p, ok := codes[d.Parent]
			if !ok {
				t.Fatalf("%q: unknown parent %q", d.Code, d.Parent)
			}
			if p.Type != Menu {
				t.Fatalf("%q: parent %q is not a menu", d.Code, d.Parent)
			}
		}
		switch d.Type {
		case Menu:
			if d.Path == "" {
				t.Fatalf("menu %q has no path", d.Code)
			}
			if strings.Contains(d.Code, ":") {
				t.Fatalf("menu code %q must not contain ':'", d.Code)
			}
		case Action:
			if strings.Count(d.Code, ":") < 1 {
				t.Fatalf("action code %q must be module:resource:action or module:action", d.Code)
			}
		default:
			t.Fatalf("%q: bad type %q", d.Code, d.Type)
		}
	}
	// every leaf menu must have its :view action registered
	hasChild := map[string]bool{}
	for _, d := range Defs {
		if d.Type == Menu && d.Parent != "" {
			hasChild[d.Parent] = true
		}
	}
	for _, d := range Defs {
		if d.Type == Menu && !hasChild[d.Code] {
			if _, ok := codes[MenuRequires(d.Code)]; !ok {
				t.Fatalf("leaf menu %q lacks action %q", d.Code, MenuRequires(d.Code))
			}
		}
	}
	for _, r := range DefaultRoles {
		for _, p := range r.Perms {
			if !IsAction(p) {
				t.Fatalf("role %q references unknown action %q", r.Code, p)
			}
		}
	}
}

func TestMenusFor(t *testing.T) {
	all := MenusFor(nil)
	if len(all) == 0 {
		t.Fatal("no menus")
	}
	only := MenusFor(func(c string) bool { return c == "system:user:view" })
	if len(only) != 1 || only[0].Code != "system" || len(only[0].Children) != 1 || only[0].Children[0].Code != "system.user" {
		t.Fatalf("unexpected filtered tree: %+v", only)
	}
	none := MenusFor(func(string) bool { return false })
	if len(none) != 0 {
		t.Fatalf("expected empty tree, got %d roots", len(none))
	}
}

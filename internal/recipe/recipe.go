// SPDX-License-Identifier: AGPL-3.0-or-later

// Package recipe reads agent recipes: Markdown files with a front matter that
// names them and their arguments, and a body of instructions for an AI agent
// with {{argument}} placeholders. The MCP endpoint offers them as prompts.
package recipe

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

type Arg struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

type Recipe struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
	Args        []Arg  `yaml:"arguments"`
	Body        string `yaml:"-"`
	File        string `yaml:"-"` // base name, for error messages
	Pro         bool   `yaml:"-"` // from the AnyProfile Pro pack
}

const maxSize = 256 << 10

var (
	nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	argRe  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,30}$`)
	holeRe = regexp.MustCompile(`\{\{\s*([a-z][a-z0-9_]*)\s*\}\}`)
)

// Parse reads one recipe file's content.
func Parse(file string, data []byte) (Recipe, error) {
	r := Recipe{File: file}
	bad := func(format string, a ...any) error {
		return apperr.Newf("recipe_invalid", "%s: %s", file, fmt.Sprintf(format, a...))
	}
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	rest, ok := bytes.CutPrefix(data, []byte("---\n"))
	if !ok {
		return r, bad("starts without front matter (---)")
	}
	head, body, ok := bytes.Cut(rest, []byte("\n---\n"))
	if !ok {
		return r, bad("front matter is not closed with ---")
	}
	if err := yaml.Unmarshal(head, &r); err != nil {
		return r, bad("front matter: %v", err)
	}
	if !nameRe.MatchString(r.Name) {
		return r, bad("name %q must be lowercase letters, digits and -", r.Name)
	}
	if strings.TrimSpace(r.Description) == "" {
		return r, bad("description is missing")
	}
	seen := map[string]bool{}
	for _, a := range r.Args {
		if !argRe.MatchString(a.Name) || seen[a.Name] {
			return r, bad("argument %q must be unique, lowercase letters, digits and _", a.Name)
		}
		seen[a.Name] = true
	}
	for _, m := range holeRe.FindAllStringSubmatch(string(body), -1) {
		if !seen[m[1]] {
			return r, bad("{{%s}} is not one of its arguments", m[1])
		}
	}
	r.Body = strings.TrimSpace(string(body))
	return r, nil
}

// Load reads every *.md in dir, sorted by name. A file that does not parse,
// or repeats a name, is left out and reported in errs.
func Load(dir string) (rs []Recipe, errs []error) {
	if dir == "" {
		return nil, nil // no folder, not the working directory
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	seen := map[string]string{}
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if info.Size() > maxSize {
			errs = append(errs, apperr.Newf("recipe_invalid", "%s: larger than 256 KB", filepath.Base(p)))
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		r, err := Parse(filepath.Base(p), data)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if other, dup := seen[r.Name]; dup {
			errs = append(errs, apperr.Newf("recipe_invalid", "%s: name %q is already used by %s", r.File, r.Name, other))
			continue
		}
		seen[r.Name] = r.File
		rs = append(rs, r)
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i].Name < rs[j].Name })
	return rs, errs
}

// Render fills the placeholders; a required argument must be given, and an
// optional one left out becomes "(not given)".
func (r Recipe) Render(args map[string]string) (string, error) {
	for _, a := range r.Args {
		if a.Required && strings.TrimSpace(args[a.Name]) == "" {
			return "", apperr.Newf("recipe_argument", "recipe %s needs %s: %s", r.Name, a.Name, a.Description)
		}
	}
	return holeRe.ReplaceAllStringFunc(r.Body, func(m string) string {
		name := holeRe.FindStringSubmatch(m)[1]
		if v := strings.TrimSpace(args[name]); v != "" {
			return v
		}
		return "(not given)"
	}), nil
}

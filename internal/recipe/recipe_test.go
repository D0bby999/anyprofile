// SPDX-License-Identifier: AGPL-3.0-or-later

package recipe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const good = "---\r\nname: youtube-upload\r\ndescription: Upload a video, Private\r\nversion: \"1.2\"\r\narguments:\r\n  - name: file\r\n    description: file in uploads\r\n    required: true\r\n  - name: title\r\n    description: video title\r\n---\r\nUpload {{file}} with title {{ title }}.\r\n"

func TestParseAndRender(t *testing.T) {
	r, err := Parse("yt.md", []byte(good))
	if err != nil {
		t.Fatal(err)
	}
	if r.Name != "youtube-upload" || r.Version != "1.2" || len(r.Args) != 2 || !r.Args[0].Required {
		t.Fatalf("recipe = %+v", r)
	}
	out, err := r.Render(map[string]string{"file": "clip.mp4"})
	if err != nil || out != "Upload clip.mp4 with title (not given)." {
		t.Fatalf("render = %q, %v", out, err)
	}
	if _, err := r.Render(map[string]string{"title": "x"}); err == nil || !strings.Contains(err.Error(), "needs file") {
		t.Fatalf("missing required: %v", err)
	}
}

func TestParseRefuses(t *testing.T) {
	cases := map[string]string{
		"no front matter": "hello",
		"unclosed":        "---\nname: a\n",
		"bad name":        "---\nname: A B\ndescription: x\n---\nbody",
		"no description":  "---\nname: a\n---\nbody",
		"unknown hole":    "---\nname: a\ndescription: x\n---\nuse {{file}}",
		"duplicate arg":   "---\nname: a\ndescription: x\narguments:\n  - name: f\n  - name: f\n---\nb",
		"bad yaml":        "---\nname: [a\n---\nb",
	}
	for name, data := range cases {
		if _, err := Parse("x.md", []byte(data)); err == nil || !strings.Contains(err.Error(), "recipe_invalid") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	write := func(name, data string) { os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600) }
	write("b.md", "---\nname: zeta\ndescription: z\n---\nz")
	write("a.md", "---\nname: alpha\ndescription: a\n---\na")
	write("c.md", "---\nname: alpha\ndescription: again\n---\na")
	write("d.md", "broken")
	write("notes.txt", "---\nname: txt\ndescription: t\n---\nt")
	rs, errs := Load(dir)
	if len(rs) != 2 || rs[0].Name != "alpha" || rs[0].File != "a.md" || rs[1].Name != "zeta" {
		t.Fatalf("recipes = %+v", rs)
	}
	if len(errs) != 2 {
		t.Fatalf("errs = %v", errs)
	}
	if rs, errs := Load(filepath.Join(dir, "missing")); len(rs) != 0 || len(errs) != 0 {
		t.Fatal("a missing folder has no recipes and no errors")
	}
}

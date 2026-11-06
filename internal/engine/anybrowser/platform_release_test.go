// SPDX-License-Identifier: AGPL-3.0-or-later

package anybrowser

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/D0bby999/anyprofile/internal/engine"
)

func TestBuiltinPlatformPin(t *testing.T) {
	r := Builtin()[0]
	if r.Version != Version || r.Chromium != Chromium || r.Catalogue != Catalogue {
		t.Fatalf("builtin does not match platform constants: %+v", r)
	}
	if platform == "windows" {
		if Version != "0.5.1" || Chromium != "155.0.8059.31" || !legacyGoogleKeyEnv ||
			strings.Contains(r.URL, "example.invalid") || !strings.Contains(r.URL, "/apostate-155.") {
			t.Fatalf("Windows upstream pin replaced by an unpublished fork: %+v", r)
		}
	} else if Version != "0.5.1.157" || Chromium != "156.0.8078.74" || legacyGoogleKeyEnv {
		t.Fatal("macOS candidate pin/keyless policy changed")
	}
}

func TestFutureReleaseDoesNotInheritLegacyGoogleEnvironment(t *testing.T) {
	a := testEngine(t)
	next := Builtin()[0]
	next.Version = "999.1"
	next.URL = "https://example.invalid/future-keyless.zip"
	a.SetCatalog(engine.NewCatalog("", Builtin()[0], next))
	p := testProfile()
	p.EngineVersion = next.Version
	spec, err := a.BuildLaunch(context.Background(), p, engine.Runtime{UserDataDir: "/u"})
	if err != nil {
		t.Fatal(err)
	}
	for _, env := range spec.Env {
		if strings.HasPrefix(env, "GOOGLE_") {
			t.Fatalf("unrecognized release inherited legacy workaround: %s", env)
		}
	}
	if len(spec.EnvUnset) != 3 {
		t.Fatal("real inherited Google credentials are not stripped")
	}
}

// Inspect Windows source on macOS too: cross-vet alone cannot catch a valid
// placeholder URL, nonexistent upstream asset name, or a metadata-only regression.
func TestWindowsLegacyTransportContract(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "release_windows.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	ast.Inspect(file, func(node ast.Node) bool {
		decl, ok := node.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, value := range decl.Values {
			literal, ok := value.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				continue
			}
			values[decl.Names[i].Name], _ = strconv.Unquote(literal.Value)
		}
		return true
	})
	want := map[string]string{
		"Version":      "0.5.1",
		"Chromium":     "155.0.8059.31",
		"zipName":      "apostate-155.0.8059.31-windows-x64.zip",
		"zipSHA256":    "8e55e44fb7f58619461e1d42a947446dcc08a2b46493c420c8e62ae27ab9c7e5",
		"binarySHA256": "5b4e065110a8a24bb35448d2716d26427a91618b0ee1f4b33723c4d25722de67",
		"frameworkSHA": "200e51e059e774e3f1674b7e143a769f111c9ba412b50303e025f2fe7dbc2360",
		"rootDir":      "apostate-155.0.8059.31-windows-x64",
	}
	for key, expected := range want {
		if values[key] != expected {
			t.Errorf("Windows %s = %q, want retained upstream %q", key, values[key], expected)
		}
	}
	workflow, err := os.ReadFile("../../../.github/workflows/windows.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workflow), "ANYBROWSER_ZIP: "+want["zipName"]) ||
		!strings.Contains(string(workflow), `\anybrowser\`+want["rootDir"]) ||
		strings.Contains(string(workflow), "anybrowser-155.") ||
		!strings.Contains(string(workflow), "go run ./tools/bench/builtin-release") ||
		!strings.Contains(string(workflow), "ANYPROFILE_TEST_ANYBROWSER_RELEASE:") {
		t.Fatal("Windows CI asset/layout does not match the real upstream pin")
	}
}

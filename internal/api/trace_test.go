// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTraceWritesOneLinePerCall(t *testing.T) {
	dir := t.TempDir()
	s := &Server{Traces: dir}
	s.trace("find", "p1", 12, nil, "")
	s.trace("click", "p1", 7, errors.New("target_not_found: no element matches text \"Lưu\""), "/tmp/shot.jpg")

	b, err := os.ReadFile(filepath.Join(dir, "mcp-"+time.Now().Format("2006-01-02")+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d: %s", len(lines), b)
	}
	var first traceLine
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if first.Tool != "find" || !first.OK || first.Who != "p1" {
		t.Fatalf("first = %+v", first)
	}
	if !strings.Contains(lines[1], "target_not_found") {
		t.Fatalf("second line = %s", lines[1])
	}
}

func TestTraceIsOptional(t *testing.T) {
	s := &Server{}
	s.trace("find", "p1", 1, nil, "") // must not panic or write anywhere
}

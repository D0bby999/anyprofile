// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// Upload is one file an agent may put into a page.
type Upload struct {
	Name string `json:"name"` // relative to the uploads folder, with /
	Size int64  `json:"size"`
}

const maxUploadsListed = 500

// ListUploads lists the regular files under dir, hidden ones and symlinks left
// out, at most 500.
func ListUploads(dir string) ([]Upload, error) {
	if dir == "" {
		return nil, apperr.New("agent_upload", "there is no uploads folder")
	}
	var out []Upload
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") && p != dir {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || len(out) >= maxUploadsListed {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		out = append(out, Upload{Name: filepath.ToSlash(rel), Size: info.Size()})
		return nil
	})
	return out, err
}

// ResolveUploads turns names into paths of regular files inside dir. Absolute
// paths, .. and symlinks that lead out of dir are refused: a page could talk
// an agent into uploading a private file, and this folder is the fence.
func ResolveUploads(dir string, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, apperr.New("agent_upload", "name at least one file from list_uploads")
	}
	if dir == "" {
		return nil, apperr.New("agent_upload", "there is no uploads folder")
	}
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		refused := apperr.Newf("agent_upload_outside", "%q is not a file in the uploads folder; see list_uploads", n)
		clean := filepath.Clean(filepath.FromSlash(n))
		if n == "" || filepath.IsAbs(n) || filepath.VolumeName(n) != "" || strings.HasPrefix(n, `\`) ||
			clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return nil, refused
		}
		real, err := filepath.EvalSymlinks(filepath.Join(root, clean))
		if err != nil {
			return nil, refused
		}
		if rel, err := filepath.Rel(root, real); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, refused
		}
		if info, err := os.Stat(real); err != nil || !info.Mode().IsRegular() {
			return nil, refused
		}
		out = append(out, real)
	}
	return out, nil
}

// UploadFile puts files (already resolved by ResolveUploads) into the file
// input ref, or into the one that opens when ref is clicked, like the
// "Select files" button of an upload dialog.
func (b *Browser) UploadFile(ctx context.Context, id, ref string, files []string) (Tab, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return Tab{}, err
	}
	if err := b.activate(ctx, t); err != nil {
		return Tab{}, err
	}
	var isInput bool
	if err := b.callOn(ctx, t, ref, `function() { return this.tagName === 'INPUT' && this.type === 'file' }`, nil, &isInput); err != nil {
		return Tab{}, err
	}
	node, f, _ := b.resolve(t, ref)
	if !isInput {
		if node, err = b.chooserInput(ctx, t, ref, len(files)); err != nil {
			return Tab{}, err
		}
	}
	if err := b.call(ctx, sessionOf(t, f), "DOM.setFileInputFiles", map[string]any{"files": files, "backendNodeId": node}, nil); err != nil {
		return Tab{}, err
	}
	pause(500*time.Millisecond, 900*time.Millisecond) // let the page take the files
	return b.describe(ctx, t), nil
}

// chooserInput clicks ref with the file chooser intercepted and returns the
// input the page opened it for. The chooser event needs the Page domain,
// which attach turns on.
func (b *Browser) chooserInput(ctx context.Context, t *tab, ref string, count int) (int64, error) {
	events, cancel := b.conn.Subscribe(64)
	defer cancel()
	if err := b.call(ctx, t.session, "Page.setInterceptFileChooserDialog", map[string]any{"enabled": true}, nil); err != nil {
		return 0, err
	}
	defer b.call(context.Background(), t.session, "Page.setInterceptFileChooserDialog", map[string]any{"enabled": false}, nil)
	if err := b.clickRef(ctx, t, ref); err != nil {
		return 0, err
	}
	wait := time.After(10 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return 0, apperr.New("agent_upload", "the browser closed")
			}
			if ev.Method != "Page.fileChooserOpened" || ev.SessionID != t.session {
				continue
			}
			var p struct {
				Mode          string `json:"mode"`
				BackendNodeID int64  `json:"backendNodeId"`
			}
			json.Unmarshal(ev.Params, &p)
			if p.BackendNodeID == 0 {
				return 0, apperr.New("agent_upload", "the page opened a file chooser without a file input")
			}
			if p.Mode == "selectSingle" && count > 1 {
				return 0, apperr.New("agent_upload", "this input takes one file")
			}
			return p.BackendNodeID, nil
		case <-wait:
			return 0, apperr.Newf("agent_upload", "clicking %s opened no file chooser; snapshot again and pick the upload button or the file input", ref)
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
}

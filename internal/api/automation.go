// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/agent"
	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/automation"
)

// actRequest is one automation step over HTTP. Profile and Device are mutually
// exclusive; a step in /v1/run may inherit them from the run.
type actRequest struct {
	Profile string             `json:"profile,omitempty"`
	Device  string             `json:"device,omitempty"`
	Tab     string             `json:"tab,omitempty"`
	Action  string             `json:"action"`
	Ref     string             `json:"ref,omitempty"`
	Target  *automation.Target `json:"target,omitempty"`

	URL       string   `json:"url,omitempty"`
	Text      string   `json:"text,omitempty"`
	Key       string   `json:"key,omitempty"`
	Option    string   `json:"option,omitempty"`
	Direction string   `json:"direction,omitempty"`
	Package   string   `json:"package,omitempty"`
	Path      string   `json:"path,omitempty"`
	X         int      `json:"x,omitempty"`
	Y         int      `json:"y,omitempty"`
	X1        int      `json:"x1,omitempty"`
	Y1        int      `json:"y1,omitempty"`
	X2        int      `json:"x2,omitempty"`
	Y2        int      `json:"y2,omitempty"`
	MS        int      `json:"ms,omitempty"`
	Pages     float64  `json:"pages,omitempty"`
	Limit     int      `json:"limit,omitempty"`
	Seconds   int      `json:"seconds,omitempty"`
	Offset    int      `json:"offset,omitempty"`
	Keep      bool     `json:"keep,omitempty"`
	Submit    bool     `json:"submit,omitempty"`
	Gone      bool     `json:"gone,omitempty"`
	Links     bool     `json:"links,omitempty"`
	FromRef   string   `json:"fromRef,omitempty"`
	ToRef     string   `json:"toRef,omitempty"`
	Steps     int      `json:"steps,omitempty"`
	Files     []string `json:"files,omitempty"`
	RequestID string   `json:"requestId,omitempty"`
	Types     []string `json:"types,omitempty"`
	Session   string   `json:"session,omitempty"`
	Secret    bool     `json:"secret,omitempty"`
}

type actReply struct {
	OK     bool `json:"ok"`
	Result any  `json:"result,omitempty"`
}

func (s *Server) act(w http.ResponseWriter, r *http.Request) {
	var in actRequest

	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, apperr.New("api_body", "the body is not JSON: "+err.Error()))
		return
	}
	if err := s.checkSession(in.Session, in); err != nil {
		writeErr(w, actStatus(err), err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	res, err := s.doAct(ctx, in)
	if err != nil {
		writeErr(w, actStatus(err), err)
		return
	}
	(&s.leases).recordStep(in.Session, in)
	writeJSON(w, http.StatusOK, actReply{OK: true, Result: res})
}

type runRequest struct {
	actRequest
	StopOnError *bool        `json:"stopOnError,omitempty"`
	Steps       []actRequest `json:"steps"`
}

func (s *Server) run(w http.ResponseWriter, r *http.Request) {
	var in runRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, apperr.New("api_body", "the body is not JSON: "+err.Error()))
		return
	}
	if len(in.Steps) == 0 {
		writeErr(w, http.StatusBadRequest, apperr.New("api_steps", "give at least one step"))
		return
	}
	if err := s.checkSession(in.Session, in.actRequest); err != nil {
		writeErr(w, actStatus(err), err)
		return
	}
	stop := true
	if in.StopOnError != nil {
		stop = *in.StopOnError
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	type stepReply struct {
		OK     bool   `json:"ok"`
		Result any    `json:"result,omitempty"`
		Error  string `json:"error,omitempty"`
	}
	out := make([]stepReply, 0, len(in.Steps))
	for i := range in.Steps {
		step := in.Steps[i]
		if step.Profile == "" {
			step.Profile = in.Profile
		}
		if step.Device == "" {
			step.Device = in.Device
		}
		if step.Session == "" {
			step.Session = in.Session
		}
		if err := s.checkSession(step.Session, step); err != nil {
			out = append(out, stepReply{OK: false, Error: err.Error()})
			if stop {
				break
			}
			continue
		}
		res, err := s.doAct(ctx, step)
		if err != nil {
			out = append(out, stepReply{OK: false, Error: err.Error()})
			if stop {
				break
			}
			continue
		}
		(&s.leases).recordStep(step.Session, step)
		out = append(out, stepReply{OK: true, Result: res})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "steps": out})
}

// findAct is GET /v1/find?profile=…&text=…&role=…&name=…&limit=10.
func (s *Server) findAct(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ref := q.Get("profile")
	if ref == "" {
		ref = q.Get("device")
	}
	target := automation.Target{
		Text: q.Get("text"), Role: q.Get("role"), Name: q.Get("name"),
		TestID: q.Get("testId"), Label: q.Get("label"), Desc: q.Get("desc"),
	}
	if target.Empty() {
		writeErr(w, http.StatusBadRequest, apperr.New("api_target", "give text, role, name, testId or label"))
		return
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	in := actRequest{Action: "find", Target: &target, Limit: limit}
	if q.Get("device") != "" {
		in.Device = ref
	} else {
		in.Profile = ref
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	res, err := s.doAct(ctx, in)
	if err != nil {
		writeErr(w, actStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"matches": res})
}

// traceTail is GET /v1/trace?lines=50: the newest MCP tool trace lines.
func (s *Server) traceTail(w http.ResponseWriter, r *http.Request) {
	if s.Traces == "" {
		writeErr(w, http.StatusNotFound, apperr.New("api_trace", "tracing is not configured"))
		return
	}
	n, _ := strconv.Atoi(r.URL.Query().Get("lines"))
	if n <= 0 || n > 500 {
		n = 50
	}
	b, err := os.ReadFile(filepath.Join(s.Traces, "mcp-"+time.Now().Format("2006-01-02")+".jsonl"))
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"lines": []string{}})
		return
	}
	all := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(all) > n {
		all = all[len(all)-n:]
	}
	writeJSON(w, http.StatusOK, map[string]any{"lines": all})
}

func (s *Server) doAct(ctx context.Context, in actRequest) (any, error) {
	if in.Action == "" {
		return nil, apperr.New("api_action", "give an action, like click or find")
	}
	if in.Profile != "" && in.Device != "" {
		return nil, apperr.New("api_surface", "give profile or device, not both")
	}
	if in.Profile != "" {
		return s.browserAct(ctx, in)
	}
	if in.Device != "" {
		return s.deviceAct(ctx, in)
	}
	return nil, apperr.New("api_surface", "give a profile or a device")
}

func (s *Server) browserAct(ctx context.Context, in actRequest) (any, error) {
	if s.Agent == nil {
		return nil, apperr.New("api_agent", "agent support is not available")
	}
	_, b, err := s.browser(in.Profile)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(in.Action) {
	case "navigate", "goto":
		return b.Navigate(ctx, in.Tab, in.URL, "")
	case "find":
		if in.Target == nil {
			return nil, apperr.New("api_target", "find needs a target")
		}
		return b.Find(ctx, in.Tab, *in.Target, in.Limit)
	case "click":
		ref, err := s.targetRef(ctx, b, in.Tab, in.Ref, in.Target)
		if err != nil {
			return nil, err
		}
		return b.Click(ctx, in.Tab, ref)
	case "click_first":
		// A page can list several controls whose names contain the wanted text
		// ("Post", "Post audience", "Boost post"); pick the one whose whole
		// name equals it, else the first match.
		if in.Target == nil {
			return nil, apperr.New("api_target", "click_first needs a target")
		}
		matches, err := b.Find(ctx, in.Tab, *in.Target, 0)
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			return nil, apperr.Newf("target_not_found", "no element matches %q after 5s", in.Target.String()).Safe()
		}
		pick := matches[0]
		for _, m := range matches {
			if strings.EqualFold(strings.TrimSpace(m.Name), strings.TrimSpace(in.Target.Name)) && in.Target.Name != "" {
				pick = m
				break
			}
			if strings.EqualFold(strings.TrimSpace(m.Name), strings.TrimSpace(in.Target.Text)) && in.Target.Text != "" {
				pick = m
				break
			}
		}
		return b.Click(ctx, in.Tab, pick.Ref)
	case "type":
		ref, err := s.targetRef(ctx, b, in.Tab, in.Ref, in.Target)
		if err != nil {
			return nil, err
		}
		return b.Type(ctx, in.Tab, ref, in.Text, !in.Keep, in.Submit)
	case "read":
		ref := in.Ref
		if in.Target != nil {
			r, err := s.targetRef(ctx, b, in.Tab, "", in.Target)
			if err != nil {
				return nil, err
			}
			ref = r
		}
		return b.ReadPage(ctx, in.Tab, ref, in.Offset, in.Links)
	case "press", "press_key":
		return b.PressKey(ctx, in.Tab, in.Key)
	case "scroll":
		return b.Scroll(ctx, in.Tab, in.Ref, in.Pages)
	case "wait":
		secs := in.Seconds
		if secs <= 0 || secs > 60 {
			secs = 10
		}
		return b.WaitFor(ctx, in.Tab, in.Text, in.Gone, in.URL, time.Duration(secs)*time.Second)
	case "hover":
		ref, err := s.targetRef(ctx, b, in.Tab, in.Ref, in.Target)
		if err != nil {
			return nil, err
		}
		return b.Hover(ctx, in.Tab, ref)
	case "double_click":
		ref, err := s.targetRef(ctx, b, in.Tab, in.Ref, in.Target)
		if err != nil {
			return nil, err
		}
		return b.DoubleClick(ctx, in.Tab, ref)
	case "right_click":
		ref, err := s.targetRef(ctx, b, in.Tab, in.Ref, in.Target)
		if err != nil {
			return nil, err
		}
		return b.RightClick(ctx, in.Tab, ref)
	case "drag":
		return b.Drag(ctx, in.Tab, in.FromRef, in.ToRef, in.X1, in.Y1, in.X2, in.Y2, in.Steps)
	case "upload":
		if len(in.Files) == 0 {
			return nil, apperr.New("api_upload", "give files from the uploads folder")
		}
		files, err := agent.ResolveUploads(s.Uploads, in.Files)
		if err != nil {
			return nil, err
		}
		ref, err := s.targetRef(ctx, b, in.Tab, in.Ref, in.Target)
		if err != nil {
			return nil, err
		}
		return b.UploadFile(ctx, in.Tab, ref, files)
	case "network":
		return b.Requests(ctx, in.Tab, in.URL, in.Types, in.Limit)
	case "body":
		return b.ResponseBody(ctx, in.Tab, in.RequestID, in.Offset)
	case "screenshot":
		img, err := b.Screenshot(ctx, in.Tab)
		if err != nil {
			return nil, err
		}
		return map[string]string{"base64": base64.StdEncoding.EncodeToString(img), "contentType": "image/jpeg"}, nil
	default:
		return nil, apperr.Newf("api_action", "unknown browser action %q", in.Action)
	}
}

func (s *Server) deviceAct(ctx context.Context, in actRequest) (any, error) {
	if s.Mobile == nil {
		return nil, apperr.New("mobile_unsupported", "this app has no mobile driver")
	}
	d, err := s.device(in.Device)
	if err != nil {
		return nil, err
	}
	c, err := s.control(d)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(in.Action) {
	case "find":
		if in.Target == nil {
			return nil, apperr.New("api_target", "find needs a target")
		}
		f, ok := c.(interface {
			Find(context.Context, automation.Target, int) ([]automation.Match, error)
		})
		if !ok {
			return nil, apperr.New("mobile_find", "finding is not supported by this driver")
		}
		return f.Find(ctx, *in.Target, in.Limit)
	case "tap":
		ref, err := deviceRef(ctx, c, in.Ref, in.Target)
		if err != nil {
			return nil, err
		}
		if ref == "" && in.X == 0 && in.Y == 0 {
			return nil, apperr.New("mobile_point", "give a ref, a target, or x and y")
		}
		if err := c.Tap(ctx, ref, in.X, in.Y); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "tap_text":
		f, ok := c.(interface {
			FindOne(context.Context, automation.Target) (automation.Match, error)
		})
		if !ok {
			return nil, apperr.New("mobile_find", "finding is not supported by this driver")
		}
		m, err := f.FindOne(ctx, automation.Target{Text: in.Text})
		if err != nil {
			return nil, err
		}
		if err := c.Tap(ctx, m.Ref, 0, 0); err != nil {
			return nil, err
		}
		return m, nil
	case "type":
		ref, err := deviceRef(ctx, c, in.Ref, in.Target)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, c.Type(ctx, ref, in.Text, in.Submit)
	case "key", "press_key":
		return map[string]any{"ok": true}, c.PressKey(ctx, in.Key)
	case "swipe":
		return map[string]any{"ok": true}, c.Swipe(ctx, in.Direction, in.X1, in.Y1, in.X2, in.Y2)
	case "long_press":
		ref, err := deviceRef(ctx, c, in.Ref, in.Target)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, c.LongPress(ctx, ref, in.X, in.Y, in.MS)
	case "drag":
		return map[string]any{"ok": true}, c.Drag(ctx, in.X1, in.Y1, in.X2, in.Y2, in.MS)
	case "launch":
		return map[string]any{"ok": true}, c.Launch(ctx, in.Package)
	case "open_url":
		return map[string]any{"ok": true}, c.OpenURL(ctx, in.URL)
	case "screenshot":
		img, err := c.Screenshot(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]string{"base64": base64.StdEncoding.EncodeToString(img), "contentType": "image/jpeg"}, nil
	case "pull":
		path, err := c.Pull(ctx, in.Path)
		if err != nil {
			return nil, err
		}
		return map[string]string{"path": path}, nil
	default:
		return nil, apperr.Newf("api_action", "unknown device action %q", in.Action)
	}
}

func actStatus(err error) int {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		switch ae.Code {
		case "api_body", "api_action", "api_steps", "api_surface", "api_target",
			"target_not_found", "target_ambiguous", "target_disabled", "mobile_ref",
			"mobile_point", "mobile_find", "agent_bad_ref", "agent_bad_target",
			"session_not_found", "session_surface":
			return http.StatusBadRequest
		default:
			return http.StatusConflict
		}
	}
	return http.StatusInternalServerError
}

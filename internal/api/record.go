// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
)

// mcpAction maps a tool name to the flow action it becomes. Observation tools
// are absent on purpose: the MCP wrapper only records non-read-only calls.
var mcpAction = map[string]string{
	"navigate": "navigate", "click": "click", "type": "type", "press_key": "press",
	"scroll": "scroll", "select_option": "select", "upload_file": "upload",
	"hover": "hover", "double_click": "double_click", "right_click": "right_click", "drag": "drag",
	"device_tap": "tap", "device_tap_text": "tap_text", "device_type": "type",
	"device_press_key": "key", "device_swipe": "swipe", "device_long_press": "long_press",
	"device_drag": "drag", "device_launch_app": "launch", "device_open_url": "open_url",
	"device_pull_file": "pull",
}

// recordMCP appends a successful MCP action to the recording lease owned by
// this MCP session. Unknown tools and open/close are skipped.
func (in mcpRecordIn) who() string {
	if in.Profile != "" {
		return in.Profile
	}
	return in.Device
}

func (s *Server) recordMCP(ctx context.Context, tool string, in any) {
	action, ok := mcpAction[tool]
	if !ok {
		return
	}
	sid := mcpSession(ctx)
	if sid == "" {
		return
	}
	b, err := json.Marshal(in)
	if err != nil {
		return
	}
	var step actRequest
	if json.Unmarshal(b, &step) != nil {
		return
	}
	step.Action = action
	step.Session = ""
	(&s.leases).appendForOwner("mcp:"+sid, step)
}

// mcpRecordIn is the surface to record.
type mcpRecordIn struct {
	Profile string `json:"profile,omitempty" jsonschema:"profile to record"`
	Device  string `json:"device,omitempty" jsonschema:"device to record instead of a profile"`
}

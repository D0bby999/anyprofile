// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package vm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

// qmpCall runs one QMP command on QEMU's control socket and returns its
// "return" value. Each call opens its own connection: QMP is idle almost
// always, and a connection per call cannot be left half-read.
func qmpCall(ctx context.Context, socket, cmd string, args any) (json.RawMessage, error) {
	var d net.Dialer
	c, err := d.DialContext(ctx, "unix", socket)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	if dl, ok := ctx.Deadline(); ok {
		c.SetDeadline(dl)
	} else {
		c.SetDeadline(time.Now().Add(10 * time.Second))
	}
	r := bufio.NewReader(c)
	if _, err := qmpRead(r); err != nil { // the greeting
		return nil, err
	}
	if _, err := qmpSend(c, r, "qmp_capabilities", nil); err != nil {
		return nil, err
	}
	return qmpSend(c, r, cmd, args)
}

type qmpReply struct {
	Return json.RawMessage `json:"return"`
	Error  *struct {
		Class string `json:"class"`
		Desc  string `json:"desc"`
	} `json:"error"`
	Event string `json:"event"`
	QMP   any    `json:"QMP"`
}

func qmpSend(c net.Conn, r *bufio.Reader, cmd string, args any) (json.RawMessage, error) {
	msg := map[string]any{"execute": cmd}
	if args != nil {
		msg["arguments"] = args
	}
	b, _ := json.Marshal(msg)
	if _, err := c.Write(append(b, '\n')); err != nil {
		return nil, err
	}
	for {
		rep, err := qmpRead(r)
		if err != nil {
			return nil, err
		}
		if rep.Event != "" {
			continue // events arrive between replies
		}
		if rep.Error != nil {
			return nil, fmt.Errorf("qmp %s: %s", cmd, rep.Error.Desc)
		}
		return rep.Return, nil
	}
}

func qmpRead(r *bufio.Reader) (qmpReply, error) {
	var rep qmpReply
	line, err := r.ReadBytes('\n')
	if err != nil {
		return rep, err
	}
	if err := json.Unmarshal(line, &rep); err != nil {
		return rep, fmt.Errorf("qmp: %w", err)
	}
	return rep, nil
}

// qmpStatus is the VM's run state, like "running" or "shutdown".
func qmpStatus(ctx context.Context, socket string) (string, error) {
	raw, err := qmpCall(ctx, socket, "query-status", nil)
	if err != nil {
		return "", err
	}
	var st struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return "", err
	}
	if st.Status == "" {
		return "", errors.New("qmp: no status")
	}
	return st.Status, nil
}

// SPDX-License-Identifier: AGPL-3.0-or-later

package ldplayer

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// runFunc runs a program and returns its trimmed output; tests replace it.
type runFunc func(ctx context.Context, bin string, args ...string) (string, error)

func execRun(ctx context.Context, bin string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.SysProcAttr = hidden()
	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(bytes.ReplaceAll(out, []byte("\r"), nil)))
	if err != nil {
		return s, fmt.Errorf("%s %s: %v: %s", "ldconsole", strings.Join(args, " "), err, s)
	}
	return s, nil
}

// console is ldconsole.exe. Every command names the instance by its name,
// never by index: LDPlayer gives a deleted instance's index to the next one.
type console struct {
	bin string
	run runFunc
}

func (c console) do(ctx context.Context, args ...string) (string, error) {
	return c.run(ctx, c.bin, args...)
}

// Instance is one line of `ldconsole list2`:
// index,title,top window,bind window,android started,pid,vbox pid[,...]
type Instance struct {
	Index   int
	Name    string
	Started bool
	PID     int
}

func parseList2(out string) []Instance {
	var all []Instance
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(strings.TrimSpace(line), ",")
		if len(f) < 6 {
			continue
		}
		idx, err := strconv.Atoi(f[0])
		if err != nil {
			continue
		}
		pid, _ := strconv.Atoi(f[5])
		all = append(all, Instance{Index: idx, Name: f[1], Started: f[4] == "1", PID: pid})
	}
	return all
}

// find is the instance called name; ok is false when there is none (deleted
// in LDPlayer itself).
func (c console) find(ctx context.Context, name string) (Instance, bool, error) {
	out, err := c.do(ctx, "list2")
	if err != nil {
		return Instance{}, false, err
	}
	for _, in := range parseList2(out) {
		if in.Name == name {
			return in, true, nil
		}
	}
	return Instance{}, false, nil
}

func (c console) running(ctx context.Context, name string) bool {
	out, err := c.do(ctx, "isrunning", "--name", name)
	return err == nil && strings.TrimSpace(out) == "running"
}

// adbPort is the instance's adb port on 127.0.0.1 (LDPlayer: 5555 + 2×index).
func adbPort(index int) int { return 5555 + 2*index }

// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package vm

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"
)

// The LineageOS virtio image boots through GRUB, which reads its settings
// from a grubenv block in the PERSIST partition. Changing it (adb without
// a key, no boot menu) needs no root and no fastboot: the block is patched in
// the device's qcow2 overlay while QEMU is stopped. qemu-img does the I/O,
// through a raw view at an offset, since qemu-img dd refuses large offsets.

const (
	grubSig   = "# GRUB Environment Block\n"
	grubSize  = 1024
	sector    = 512
	gptHeader = 34 * sector
)

// grubDefaults is what every device boots with: adb on without a key
// (decision 7 of the plan), no boot menu, and the boot arguments the PoC needed
// under HVF (no Wi-Fi HAL, no BTI).
var grubDefaults = map[string]string{
	"android_insecure_adb": "1",
	"grub_timeout":         "0",
	"grub_android_default": "android",
	"android_wifi_impl":    "none arm64.nobti",
}

type disk struct {
	qemuImg string
	path    string
}

// readRaw copies size bytes at offset out of the disk.
func (d disk) readRaw(ctx context.Context, offset, size int64) ([]byte, error) {
	tmp, err := os.MkdirTemp("", "ap-grub")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	out := filepath.Join(tmp, "raw")
	opts := fmt.Sprintf("driver=raw,offset=%d,size=%d,file.driver=qcow2,file.file.filename=%s", offset, size, optEscape(d.path))
	if b, err := exec.CommandContext(ctx, d.qemuImg, "convert", "-O", "raw", "--image-opts", opts, out).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("qemu-img read: %v: %s", err, bytes.TrimSpace(b))
	}
	return os.ReadFile(out)
}

// writeRaw puts b at offset in the disk.
func (d disk) writeRaw(ctx context.Context, offset int64, b []byte) error {
	tmp, err := os.MkdirTemp("", "ap-grub")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	in := filepath.Join(tmp, "raw")
	if err := os.WriteFile(in, b, 0o600); err != nil {
		return err
	}
	opts := fmt.Sprintf("driver=raw,offset=%d,size=%d,file.driver=qcow2,file.file.filename=%s", offset, len(b), optEscape(d.path))
	if out, err := exec.CommandContext(ctx, d.qemuImg, "convert", "-n", "-f", "raw", in, "--target-image-opts", opts).CombinedOutput(); err != nil {
		return fmt.Errorf("qemu-img write: %v: %s", err, bytes.TrimSpace(out))
	}
	return nil
}

// optEscape doubles commas, which separate --image-opts options.
func optEscape(p string) string { return strings.ReplaceAll(p, ",", ",,") }

// partition finds a GPT partition by name: its first sector and length.
func partition(gpt []byte, name string) (first, sectors int64, err error) {
	if len(gpt) < 2*sector || string(gpt[sector:sector+8]) != "EFI PART" {
		return 0, 0, errors.New("no GPT header")
	}
	lba := binary.LittleEndian.Uint64(gpt[sector+72:])
	n := binary.LittleEndian.Uint32(gpt[sector+80:])
	size := binary.LittleEndian.Uint32(gpt[sector+84:])
	if size < 128 {
		return 0, 0, errors.New("bad GPT entry size")
	}
	for i := uint32(0); i < n; i++ {
		at := int64(lba)*sector + int64(i)*int64(size)
		if at+128 > int64(len(gpt)) {
			break
		}
		e := gpt[at : at+128]
		u := make([]uint16, 36)
		for j := range u {
			u[j] = binary.LittleEndian.Uint16(e[56+2*j:])
		}
		got := strings.TrimRight(string(utf16.Decode(u)), "\x00")
		if strings.EqualFold(got, name) {
			f := binary.LittleEndian.Uint64(e[32:])
			l := binary.LittleEndian.Uint64(e[40:])
			return int64(f), int64(l-f) + 1, nil
		}
	}
	return 0, 0, fmt.Errorf("no %s partition", name)
}

// parseGrubenv splits a block into its comment lines and its variables.
func parseGrubenv(block []byte) (comments []string, env map[string]string, err error) {
	if !bytes.HasPrefix(block, []byte(grubSig)) {
		return nil, nil, errors.New("not a grubenv block")
	}
	env = map[string]string{}
	for _, l := range strings.Split(string(bytes.TrimRight(block[len(grubSig):], "#")), "\n") {
		switch {
		case l == "":
		case strings.HasPrefix(l, "#"):
			comments = append(comments, l)
		default:
			k, v, _ := strings.Cut(l, "=")
			env[k] = v
		}
	}
	return comments, env, nil
}

// renderGrubenv writes a block back, padded with '#' to 1024 bytes.
func renderGrubenv(comments []string, env map[string]string) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(grubSig)
	for _, c := range comments {
		b.WriteString(c + "\n")
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.ContainsAny(k+env[k], "\n") || strings.Contains(k, "=") {
			return nil, fmt.Errorf("grubenv: bad variable %q", k)
		}
		b.WriteString(k + "=" + env[k] + "\n")
	}
	if b.Len() > grubSize {
		return nil, errors.New("grubenv too large")
	}
	b.Write(bytes.Repeat([]byte("#"), grubSize-b.Len()))
	return b.Bytes(), nil
}

// setGrubenv updates variables in the disk's PERSIST grubenv and checks the
// write by reading it back. QEMU must not have the disk open.
func setGrubenv(ctx context.Context, d disk, set map[string]string) error {
	gpt, err := d.readRaw(ctx, 0, gptHeader)
	if err != nil {
		return err
	}
	first, sectors, err := partition(gpt, "persist")
	if err != nil {
		return err
	}
	part, err := d.readRaw(ctx, first*sector, sectors*sector)
	if err != nil {
		return err
	}
	at := bytes.Index(part, []byte(grubSig))
	if at < 0 || at+grubSize > len(part) {
		return errors.New("no grubenv block in the persist partition")
	}
	comments, env, err := parseGrubenv(part[at : at+grubSize])
	if err != nil {
		return err
	}
	for k, v := range set {
		env[k] = v
	}
	block, err := renderGrubenv(comments, env)
	if err != nil {
		return err
	}
	off := first*sector + int64(at)
	if err := d.writeRaw(ctx, off, block); err != nil {
		return err
	}
	check, err := d.readRaw(ctx, off, grubSize)
	if err != nil {
		return err
	}
	if !bytes.Equal(check, block) {
		return errors.New("grubenv did not read back as written")
	}
	return nil
}

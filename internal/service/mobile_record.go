// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// mobileRecording is one screen recording running on a device.
type mobileRecording struct {
	remote string
	done   chan struct{}
	err    error
}

// StartMobileRecording records the device screen with Android's screenrecord,
// capped at three minutes. One recording per device.
func (s *Service) StartMobileRecording(ctx context.Context, id string) error {
	c, err := s.MobileControl(id)
	if err != nil {
		return err
	}
	s.recMu.Lock()
	if s.recordings == nil {
		s.recordings = map[string]*mobileRecording{}
	}
	if s.recordings[id] != nil {
		s.recMu.Unlock()
		return apperr.New("mobile_recording", "this device is already recording")
	}
	remote := fmt.Sprintf("/sdcard/anyprofile-%d.mp4", time.Now().Unix())
	rec := &mobileRecording{remote: remote, done: make(chan struct{})}
	s.recordings[id] = rec
	s.recMu.Unlock()
	go func() {
		defer close(rec.done)
		_, rec.err = c.ADB.Shell(context.Background(), "screenrecord --time-limit 180 --bit-rate 8000000 "+remote)
	}()
	return nil
}

// StopMobileRecording stops the recorder, pulls the MP4 into
// ~/Movies/AnyProfile, deletes the device copy, and opens it.
func (s *Service) StopMobileRecording(ctx context.Context, id string) (string, error) {
	s.recMu.Lock()
	rec := s.recordings[id]
	delete(s.recordings, id)
	s.recMu.Unlock()
	if rec == nil {
		return "", apperr.New("mobile_not_recording", "this device is not recording")
	}
	c, err := s.MobileControl(id)
	if err != nil {
		return "", err
	}
	_, _ = c.ADB.Shell(ctx, "pkill -INT screenrecord")
	select {
	case <-rec.done:
	case <-time.After(10 * time.Second):
	}
	d, err := s.liveMobile(id)
	if err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Movies", "AnyProfile")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	local := filepath.Join(dir, safeFileName(d.Name)+"-"+time.Now().Format("20060102-150405")+".mp4")
	if _, err := c.ADB.Run(ctx, "pull", rec.remote, local); err != nil {
		return "", apperr.Newf("mobile_record_pull", "could not copy the recording: %v", err)
	}
	_, _ = c.ADB.Shell(ctx, "rm -f "+rec.remote)
	openPath(local)
	return local, nil
}

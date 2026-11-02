// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"

	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/store"
)

// PublishTasks is the queue the "Lịch đăng" page shows, newest first.
func (a *App) PublishTasks() ([]store.PublishTask, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.ListPublishTasks("")
}

// CreatePublishTask queues one post. The file must be in the uploads folder.
func (a *App) CreatePublishTask(in service.PublishInput) (store.PublishTask, error) {
	if err := a.ready(); err != nil {
		return store.PublishTask{}, err
	}
	return a.svc.CreatePublishTask(context.Background(), in)
}

func (a *App) CancelPublishTask(id string) error {
	return a.must(func() error { return a.svc.CancelPublishTask(id) })
}

func (a *App) RetryPublishTask(id string) error {
	return a.must(func() error { return a.svc.RetryPublishTask(id) })
}

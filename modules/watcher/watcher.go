// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package watcher

import (
	"context"
	"io/fs"
	"os"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"

	"github.com/syncthing/notify"
)

type CreateWatcherOpts struct {
	PathsCallback   func(func(path, name string, d fs.DirEntry, err error) error) error
	BeforeCallback  func()
	BetweenCallback func()
	AfterCallback   func()
}

func CreateWatcher(ctx context.Context, desc string, opts *CreateWatcherOpts) {
	go run(ctx, desc, opts)
}

func run(ctx context.Context, desc string, opts *CreateWatcherOpts) {
	if opts.BeforeCallback != nil {
		opts.BeforeCallback()
	}
	if opts.AfterCallback != nil {
		defer opts.AfterCallback()
	}
	ctx, _, finished := process.GetManager().AddTypedContext(ctx, "Watcher: "+desc, process.SystemProcessType, true)
	defer finished()

	log.Trace("Watcher loop starting for %s", desc)
	defer log.Trace("Watcher loop ended for %s", desc)

	// Make the channel buffered to ensure no event is dropped. Notify will drop
	// an event if the receiver is not able to keep up the sending pace.
	events := make(chan notify.EventInfo, 1)

	if err := opts.PathsCallback(func(path, _ string, _ fs.DirEntry, err error) error {
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		log.Trace("Watcher: %s watching %q", desc, path)
		if err := notify.Watch(path, events, notify.All); err != nil {
			log.Trace("Watcher: %s unable to watch %q: error %v", desc, path, err)
		}
		return nil
	}); err != nil {
		log.Error("Unable to create watcher for %s: %v", desc, err)
		notify.Stop(events)
		return
	}

	// Note we don't call the BetweenCallback here

	for {
		select {
		case event, ok := <-events:
			if !ok {
				notify.Stop(events)
				return
			}

			log.Debug("Watched file for %s had event: %v", desc, event)
		case <-ctx.Done():
			notify.Stop(events)
			return
		}

		// Recreate the watcher - only call the BetweenCallback after the new watcher is set-up
		notify.Stop(events)
		events = make(chan notify.EventInfo, 1)

		if err := opts.PathsCallback(func(path, _ string, _ fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			log.Trace("Watcher: %s watching %q", desc, path)
			if err := notify.Watch(path, events, notify.All); err != nil {
				log.Trace("Watcher: %s unable to watch %q: error %v", desc, path, err)
			}
			return nil
		}); err != nil {
			log.Error("Unable to create watcher for %s: %v", desc, err)
			notify.Stop(events)
			return
		}

		// Inform our BetweenCallback that there has been an event
		if opts.BetweenCallback != nil {
			opts.BetweenCallback()
		}
	}
}

// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package watcher

import (
	"context"
	"io/fs"
	"os"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"

	"github.com/fsnotify/fsnotify"
)

// CreateWatcherOpts are options to configure the watcher
type CreateWatcherOpts struct {
	// PathsCallback is used to set the required paths to watch
	PathsCallback func(func(path, name string, d fs.DirEntry, err error) error) error

	// BeforeCallback is called before any files are watched
	BeforeCallback func()

	// Between Callback is called between after a watched event has occurred
	BetweenCallback func()

	// AfterCallback is called as this watcher ends
	AfterCallback func()
}

// CreateWatcher creates a watcher labelled with the provided description and running with the provided options.
// The created watcher will create a subcontext from the provided ctx and register it with the process manager.
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

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error("Unable to create watcher for %s: %v", desc, err)
		return
	}
	if err := opts.PathsCallback(func(path, _ string, d fs.DirEntry, err error) error {
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		log.Trace("Watcher: %s watching %q", desc, path)
		_ = watcher.Add(path)
		return nil
	}); err != nil {
		log.Error("Unable to create watcher for %s: %v", desc, err)
		_ = watcher.Close()
		return
	}

	// Note we don't call the BetweenCallback here

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				_ = watcher.Close()
				return
			}
			log.Debug("Watched file for %s had event: %v", desc, event)
		case err, ok := <-watcher.Errors:
			if !ok {
				_ = watcher.Close()
				return
			}
			log.Error("Error whilst watching files for %s: %v", desc, err)
		case <-ctx.Done():
			_ = watcher.Close()
			return
		}

		// Recreate the watcher - only call the BetweenCallback after the new watcher is set-up
		_ = watcher.Close()
		watcher, err = fsnotify.NewWatcher()
		if err != nil {
			log.Error("Unable to create watcher for %s: %v", desc, err)
			return
		}
		if err := opts.PathsCallback(func(path, _ string, _ fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			_ = watcher.Add(path)
			return nil
		}); err != nil {
			log.Error("Unable to create watcher for %s: %v", desc, err)
			_ = watcher.Close()
			return
		}

		// Inform our BetweenCallback that there has been an event
		if opts.BetweenCallback != nil {
			opts.BetweenCallback()
		}
	}
}

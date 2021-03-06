package watcher

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
)

// File is a watcher implementation to watch the change for a single file.
// The code of this file is largely borrowed from koanf
// (https://github.com/knadh/koanf/blob/master/providers/file/file.go)
// The original implementation doesn't support context, so we have to fork and make changes downstream.
// License: https://github.com/knadh/koanf/blob/master/LICENSE
type File struct {
	Path string
}

// Watch watches the change to the file. If the file is edited or created, the reload function will be called.
// note the reload function should not just load the changes made within this file, but rather it should reload
// the whole config stack. For example, if the flag or env takes precedence over the config file, they should remain
// to be so after the file changes.
func (f File) Watch(ctx context.Context, reload func() error) error {
	// Resolve symlinks and save the original path so that changes to symlinks
	// can be detected.
	realPath, err := filepath.EvalSymlinks(f.Path)
	if err != nil {
		return err
	}
	realPath = filepath.Clean(realPath)

	// Although only a single file is being watched, fsnotify has to watch
	// the whole parent directory to pick up all events such as symlink changes.
	fDir, _ := filepath.Split(f.Path)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	var (
		lastEvent     string
		lastEventTime time.Time
	)

	err = w.Add(fDir)
	if err != nil {
		return errors.Wrap(err, "unable to add watch dir")
	}

	for {
		select {
		case event, ok := <-w.Events:
			if !ok {
				return errors.New("fsnotify watch channel closed")
			}

			// Use a simple timer to buffer events as certain events fire
			// multiple times on some platforms.
			if event.String() == lastEvent && time.Since(lastEventTime) < time.Millisecond*5 {
				continue
			}
			lastEvent = event.String()
			lastEventTime = time.Now()

			evFile := filepath.Clean(event.Name)

			// Since the event is triggered on a directory, is this
			// one on the file being watched?
			if evFile != realPath && evFile != f.Path {
				continue
			}

			// The file was removed.
			if event.Op&fsnotify.Remove != 0 {
				return fmt.Errorf("file %s was removed", event.Name)
			}

			// Resolve symlink to get the real path, in case the symlink's
			// target has changed.
			curPath, err := filepath.EvalSymlinks(f.Path)
			if err != nil {
				return err
			}
			realPath = filepath.Clean(curPath)

			// Finally, we only care about create and write.
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			// Trigger event.
			if err = reload(); err != nil {
				return err
			}

		// There's an error.
		case err, ok := <-w.Errors:
			if !ok {
				return errors.New("fsnotify err channel closed")
			}

			return err
		case <-ctx.Done():
			return nil
		}
	}
}

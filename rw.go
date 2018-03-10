/* RecursiveWatcher
 * Watches over a directory and all its childrens for changes
 * and fire up the event handler or error handler accordingly
 */
package main

import (
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"
)

type RecursiveWatcher struct {
	watcher     *fsnotify.Watcher
	handleEvent func(fsnotify.Event)
	handleError func(error)
}

func (rw *RecursiveWatcher) AddRecursive(name string) error {
	filepath.Walk(name, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := rw.Add(path); err != nil {
				glog.Errorf("Cannot add path %s; err: %s", path, err)
				return err
			}
		}
		return nil
	})
	return nil
}

func (rw *RecursiveWatcher) Add(name string) error {
	glog.Infof("Watching path: %s", name)
	return rw.watcher.Add(name)
}

func (rw *RecursiveWatcher) Close() error {
	glog.Infof("Closing RecursiveWatcher")
	return rw.watcher.Close()
}

func (rw *RecursiveWatcher) Remove(name string) error {
	glog.Infof("Stopped watching path: %s", name)
	return rw.watcher.Remove(name)
}

func (rw *RecursiveWatcher) RegisterEventHandler(eventHandler func(fsnotify.Event)) {
	glog.Warningf("Registering EventHandler")
	rw.handleEvent = eventHandler
}

func (rw *RecursiveWatcher) RegisterErrorHandler(errorHandler func(error)) {
	glog.Warningf("Registering ErrorHandler")
	rw.handleError = errorHandler
}

func (rw *RecursiveWatcher) PushEvent(event fsnotify.Event) {
	rw.watcher.Events <- event
}

func (rw *RecursiveWatcher) Start() {
	for {
		select {
		case event := <-rw.watcher.Events:
			rw.handleEvent(event)
		case err := <-rw.watcher.Errors:
			rw.handleError(err)
		}
	}
}

func NewRecursiveWatcher() (*RecursiveWatcher, error) {
	var err error
	rw := RecursiveWatcher{}
	rw.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	glog.Warningf("Created RecursiveWatcher")
	return &rw, nil
}

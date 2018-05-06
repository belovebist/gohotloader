/* RecursiveWatcher
 * Watches over a directory and all its childrens for changes
 * and fire up the event handler or error handler accordingly
 */
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
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
				log.Printf("Cannot add path %s; err: %s", path, err)
				return err
			}
		}
		return nil
	})
	return nil
}

func (rw *RecursiveWatcher) Add(name string) error {
	log.Printf("Watching path: %s", name)
	return rw.watcher.Add(name)
}

func (rw *RecursiveWatcher) Close() error {
	log.Printf("Closing RecursiveWatcher")
	return rw.watcher.Close()
}

func (rw *RecursiveWatcher) Remove(name string) error {
	log.Printf("Stopped watching path: %s", name)
	return rw.watcher.Remove(name)
}

func (rw *RecursiveWatcher) RegisterEventHandler(eventHandler func(fsnotify.Event)) {
	log.Printf("Registering EventHandler")
	rw.handleEvent = eventHandler
}

func (rw *RecursiveWatcher) RegisterErrorHandler(errorHandler func(error)) {
	log.Printf("Registering ErrorHandler")
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
	log.Printf("Created RecursiveWatcher")
	return &rw, nil
}

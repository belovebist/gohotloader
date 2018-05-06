/*
* Usage:
* gohotloader -watch=csv/of/dirs/to/watch/inside/$GOPATH/src -build=name/of/dir/to/build/inside/$GOPATH/src -exec=path/to/built/executable
 */

package main

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"

	"gopkg.in/urfave/cli.v1"
)

func main() {
	app := cli.NewApp()
	app.Name = "Go HotLoader"
	app.Usage = "Build and reload app when code is changed"

	app.Flags = []cli.Flag{
		cli.StringSliceFlag{
			Name:  "watch, w",
			Usage: "Directory or file path to watch for changes; If Glob Pattern is provided, it must be quoted; Eg: -w \"/home/deps/*\"",
		},
		cli.StringFlag{
			Name:  "app, a",
			Usage: "Path to application source code directory to build and run; Must be inside $GOPATH/src",
		},
	}
	app.Action = func(c *cli.Context) error {
		appPath := c.String("app")

		pathsToWatchArg := c.StringSlice("watch")
		pathsToWatch := []string{}
		for _, pathToWatchArg := range pathsToWatchArg {
			pathToWatch, err := filepath.Glob(pathToWatchArg)
			if err != nil {
				glog.Warningf("WATCH; Invalid path: %v; skipping", pathToWatchArg)
				continue
			}
			pathsToWatch = append(pathsToWatch, pathToWatch...)
		}

		hotLoader := new(HotLoader)
		hotLoader.AppPath = appPath
		hotLoader.PathsToWatch = pathsToWatch
		hotLoader.execPath = "/tmp/hl_build"
		return hotLoader.Start()
	}
	err := app.Run(os.Args)
	if err != nil {
		glog.Fatal(err)
	}
}

type Config map[string]interface{}

var config Config

type HotLoader struct {
	PathsToWatch []string
	AppPath      string
	execPath     string
	rw           *RecursiveWatcher //  recursive watcher to monitor the config["watch"] directories
	cmd          *exec.Cmd         // command that's running the watched application
}

// execute and handle basic errors
func (hl *HotLoader) exec(name string, args ...string) (*exec.Cmd, io.ReadCloser, io.ReadCloser, error) {
	cmd := exec.Command(name, args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, errors.New("exec; stderr; " + err.Error())
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, errors.New("exec; stdout; " + err.Error())
	}

	err = cmd.Start()
	if err != nil {
		return nil, nil, nil, errors.New("exec; start; " + err.Error())
	}
	return cmd, stderr, stdout, nil
}

// go get all the dependencies before building
func (hl *HotLoader) goGet() error {
	cmd, stderr, stdout, err := hl.exec("go", "get", hl.AppPath)
	if err != nil {
		return errors.New("goGet; " + err.Error())
	}

	io.Copy(os.Stdout, stdout)
	errBuf, _ := ioutil.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		return errors.New("goGet; " + string(errBuf))
	}
	return nil
}

// go build
func (hl *HotLoader) build() error {
	if err := hl.goGet(); err != nil {
		return errors.New("build; " + err.Error())
	}
	cmd, stderr, stdout, err := hl.exec("go",
		"build", "-o", hl.execPath, hl.AppPath)
	if err != nil {
		return errors.New("build; " + err.Error())
	}

	io.Copy(os.Stdout, stdout)
	errBuf, _ := ioutil.ReadAll(stderr)

	if err := cmd.Wait(); err != nil {
		return errors.New("build; " + string(errBuf))
	}
	return nil
}

// run the built executable
func (hl *HotLoader) run() error {
	cmd, stderr, stdout, err := hl.exec(hl.execPath)
	if err != nil {
		return errors.New("run; " + err.Error())
	}
	go io.Copy(os.Stderr, stderr)
	go io.Copy(os.Stdout, stdout)

	hl.cmd = cmd
	return nil
}

func (hl *HotLoader) reload() error {
	if hl.cmd != nil {
		glog.Infof("reload; killing pid: %d", hl.cmd.Process.Pid)
		hl.cmd.Process.Kill()
	}
	return hl.run()
}

func (hl *HotLoader) startWatcher() {

	hl.rw.RegisterEventHandler(func(event fsnotify.Event) {
		rebuild := false
		if event.Op&fsnotify.Remove == fsnotify.Remove {
			hl.rw.Remove(event.Name)
			rebuild = true
		}
		if event.Op&fsnotify.Rename == fsnotify.Rename {
			rebuild = true
		}
		if event.Op&fsnotify.Write == fsnotify.Write {
			rebuild = true
		}
		if event.Op&fsnotify.Create == fsnotify.Create {
			hl.rw.Add(event.Name)
			rebuild = true
		}

		if rebuild {
			glog.Warningf("Building %s", hl.AppPath)
			if err := hl.build(); err != nil {
				glog.Errorf("BUILD; Failed; %s" + err.Error())
			} else {
				glog.Warningf("Reloading %s", hl.execPath)
				if err := hl.reload(); err != nil {
					glog.Errorf("RELOAD; Failed; %s" + err.Error())
				}
			}
		}
	})

	hl.rw.RegisterErrorHandler(func(err error) {})

	hl.rw.Start()
}

// Start HotLoader itself
func (hl *HotLoader) Start() error {
	glog.Warningf("Starting HotLoader; BUILD: %v; WATCH: %v", hl.AppPath, hl.PathsToWatch)

	rw, err := NewRecursiveWatcher()
	if err != nil {
		return err
	}
	hl.rw = rw

	defer rw.Close()
	done := make(chan bool)

	go hl.startWatcher() // start the watcher

	for _, dir := range hl.PathsToWatch {
		if err := hl.rw.AddRecursive(dir); err != nil {
			return err
		}
	}

	// Trigger start event for the first time
	hl.rw.PushEvent(fsnotify.Event{
		Name: "StartEvent",
		Op:   fsnotify.Write,
	})

	<-done
	return nil
}

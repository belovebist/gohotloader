/*
* Usage:
* gohotloader -watch=csv/of/dirs/to/watch/inside/$GOPATH/src -build=name/of/dir/to/build/inside/$GOPATH/src -exec=path/to/built/executable
 */

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"
)

func main() {
	hotLoader := new(HotLoader)
	hotLoader.ParseArgs()
	hotLoader.Start()
}

type Config map[string]interface{}

var config Config

type HotLoader struct {
	config Config
	rw     *RecursiveWatcher //  recursive watcher to monitor the config["watch"] directories
	cmd    *exec.Cmd         // command that's running the watched application
}

func (hl *HotLoader) ParseArgs() {
	watch := flag.String("watch", "", "Comma separated list of dirs to watch inside $GOPATH/src")
	build := flag.String("build", "", "Build particular dir inside $GOPATH/src")
	exec := flag.String("exec", "/tmp/hl_build", "Path to the built executable")
	flag.Parse()
	if *build == "" {
		*build = *watch
	}

	if *watch == "" {
		glog.Error("Must provide option -watch (dirs to watch, inside $GOPATH)")
		flag.Usage()
		os.Exit(1)
	}
	if *build == "" {
		glog.Error("Must provide option -build (dir to build, inside $GOPATH)")
		flag.Usage()
		os.Exit(1)
	}
	hl.config = Config{
		"watch": strings.Split(*watch, ","),
		"build": *build,
		"exec":  *exec,
	}
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
	cmd, stderr, stdout, err := hl.exec("go", "get", hl.config["build"].(string))
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
		"build", "-o", hl.config["exec"].(string), hl.config["build"].(string))
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
	cmd, stderr, stdout, err := hl.exec(hl.config["exec"].(string))
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
			glog.Warningf("Building %s", hl.config["build"].(string))
			if err := hl.build(); err != nil {
				glog.Errorf("BUILD FAILED; %s" + err.Error())
			} else {
				glog.Warningf("Reloading %s", hl.config["exec"].(string))
				if err := hl.reload(); err != nil {
					glog.Errorf("RELOAD FAILED; %s" + err.Error())
				}
			}
		}
	})

	hl.rw.RegisterErrorHandler(func(err error) {})

	hl.rw.Start()
}

// Start HotLoader itself
func (hl *HotLoader) Start() {
	glog.Warningf("Starting HotLoader; build: %v", hl.config["build"])

	rw, err := NewRecursiveWatcher()
	if err != nil {
		glog.Fatalf("Start; %s", err)
	}
	hl.rw = rw

	defer rw.Close()
	done := make(chan bool)

	go hl.startWatcher() // start the watcher

	for _, dir := range hl.config["watch"].([]string) {
		gopath, ok := os.LookupEnv("GOPATH")
		if !ok {
			gopath = "/go"
		}
		dir = fmt.Sprintf("%s/src/%s", gopath, dir)
		if err := hl.rw.AddRecursive(dir); err != nil {
			glog.Fatalf("Start; %s", err)
		}
	}

	// Trigger start event for the first time
	hl.rw.PushEvent(fsnotify.Event{
		Name: "StartEvent",
		Op:   fsnotify.Write,
	})

	<-done
}

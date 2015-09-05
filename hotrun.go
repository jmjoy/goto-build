package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/fsnotify.v1"
)

const (
	VERSION = "v0.1"
)

// flag args
var (
	gBuildCmd string
	gRunCmd   string
	gIsHelp   bool
)

var (
	gIsWaiting bool
	gPrevious  time.Time
	gMutex     sync.Mutex
)

func init() {
	flag.StringVar(&gBuildCmd, "build-cmd", "", "Specify build command")
	flag.StringVar(&gRunCmd, "run-cmd", "", "Specify run command")
	flag.BoolVar(&gIsHelp, "h", false, "Show help info")

	flag.Usage = func() {
		fmt.Println("auto-build [options] [directory]")
		flag.PrintDefaults()
	}
}

func main() {
	parseArg()

	watcher, err := initWatcher()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer watcher.Close()

	handleGoSourceChange(watcher)
}

func parseArg() {
	flag.Parse()

	if gIsHelp {
		flag.Usage()
		os.Exit(0)
	}

	// current work directory
	wd := flag.Arg(0)
	if wd != "" {
		if err := os.Chdir(wd); err != nil {
			fmt.Println(err)
			return
		}
	}

	// set build command
	if gBuildCmd == "" {
		gBuildCmd = "go build"
	}

	// set run command
	if gRunCmd == "" {
		cwd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		gRunCmd = filepath.Join(cwd, filepath.Base(cwd))
	}
}

func initWatcher() (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	fmt.Printf(">> Watching [%s]\n\n", cwd)

	watchDirs := make(map[string]struct{}) // hashset
	getRecursiveDirs(cwd, watchDirs)

	for dir := range watchDirs {
		watcher.Add(dir)
	}

	return watcher, nil
}

func handleGoSourceChange(watcher *fsnotify.Watcher) {
loop:
	for {
		select {
		case event := <-watcher.Events:

			if !isFileChanged(event.Op) {
				continue loop
			}

			if filepath.Ext(event.Name) != ".go" {
				continue loop
			}

			if gIsWaiting {
				continue loop
			}

			go func() {
				gMutex.Lock()
				defer gMutex.Unlock()

				if time.Now().Sub(gPrevious) < time.Second {
					return
				}
				gPrevious = time.Now()

				fmt.Printf(">> File [%s] has changed!\n\n", event.Name)
				buildAndRun()
			}()

		case err := <-watcher.Errors:

			fmt.Printf(">> WATCH ERROr: %s\n\n", err.Error())

		}
	}
}

func getRecursiveDirs(dir string, dirs map[string]struct{}) error {
	dirs[dir] = struct{}{}

	fInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, fInfo := range fInfos {
		if !fInfo.IsDir() {
			continue
		}
		if strings.HasPrefix(fInfo.Name(), ".") {
			continue
		}
		err = getRecursiveDirs(filepath.Join(dir, fInfo.Name()), dirs)
		if err != nil {
			return err
		}
	}

	return nil
}

func isFileChanged(op fsnotify.Op) bool {
	if op&fsnotify.Write == fsnotify.Write {
		return true
	}
	if op&fsnotify.Create == fsnotify.Create {
		return true
	}
	return false
}

func buildAndRun() {
	gIsWaiting = true
	defer func() {
		gIsWaiting = false
	}()

	// build
	cmd, err := execCommand(gBuildCmd)
	if err != nil {
		fmt.Printf(">> BUILD ERROR: %s\n\n", err.Error())
		return
	}
	fmt.Printf(">> BUILD SUCCESS\n\n")

	fmt.Printf(">> Restarting...\n\n")

	// stop
	if cmd != nil && cmd.Process != nil {
		err := cmd.Process.Kill()
		if err != nil && err.Error() != "os: process already finished" {
			fmt.Printf(">> KILL ERROR: %s\n\n", err.Error())
			return
		}
	}

	// start
	go execCommand(gRunCmd)
}

func execCommand(command string) (*exec.Cmd, error) {
	cmdSlice := strings.Split(command, " ")
	if len(cmdSlice) == 0 {
		panic("command format error: " + gBuildCmd)
	}
	cmd := exec.Command(cmdSlice[0], cmdSlice[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd, cmd.Run()
}

func logStatus() {

}

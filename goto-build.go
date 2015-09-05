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

	"github.com/issue9/term/colors"
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

	gLastCmd *exec.Cmd
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

	logStatus(INFO, "Watching [%s]\n", cwd)

	watchDirs := make(map[string]struct{}) // hashset
	getRecursiveDirs(cwd, watchDirs)

	for dir := range watchDirs {
		watcher.Add(dir)
	}

	return watcher, nil
}

func handleGoSourceChange(watcher *fsnotify.Watcher) {
	// first
	buildAndRun()

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

				logStatus(INFO, "File [%s] has changed!\n", event.Name)
				buildAndRun()
			}()

		case err := <-watcher.Errors:

			logStatus(ERROR, "WATCH ERROR: %s\n", err.Error())
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
	cmd := execCommand(gBuildCmd)
	err := cmd.Run()
	if err != nil {
		logStatus(ERROR, "BUILD ERROR: %s\n", err.Error())
		return
	}
	logStatus(SUCCESS, "BUILD SUCCESS\n")

	logStatus(INFO, "Restarting...\n")

	// stop
	if gLastCmd != nil {
		err := gLastCmd.Process.Kill()
		if err != nil {
			logStatus(ERROR, "KILL ERROR: %s\n", err.Error())
			return
		}
	}

	gLastCmd = execCommand(gRunCmd)
	go gLastCmd.Run()
}

func execCommand(command string) *exec.Cmd {
	cmdSlice := strings.Split(command, " ")
	if len(cmdSlice) == 0 {
		panic("command format error: " + gBuildCmd)
	}
	cmd := exec.Command(cmdSlice[0], cmdSlice[1:]...)
	// cmd.Stdin = os.Stdin // 使Command.Process不能正常Kill掉的罪魁祸首
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd
}

type LogLevel int

const (
	SUCCESS = iota
	INFO
	ERROR
)

func logStatus(lv LogLevel, format string, args ...interface{}) {
	var color colors.Color
	var status string

	switch lv {
	case SUCCESS:
		color = colors.Green
		status = "SUCCESS"

	case INFO:
		color = colors.Blue
		status = "INFO"

	case ERROR:
		color = colors.Red
		status = "ERROR"

	default:
		panic("Undefined LogLevel")
	}

	colors.Print(colors.Stdout, color, colors.Default, "["+status+"]")
	colors.Printf(colors.Stdout, colors.Default, colors.Default, " "+format, args...)
}

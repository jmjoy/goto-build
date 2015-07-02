package main

import (
	"container/list"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/fsnotify.v1"
)

func main() {
	parseArgs()

	if gDir == "" {
		gDir, _ = os.Getwd()
	}

	fInfo, err := os.Stat(gDir)
	if err != nil {
		handlerFatalErr(err)
		return
	}
	if !fInfo.IsDir() {
		handlerFatalErr(fmt.Errorf("Fatal: the path isn't a directory"))
		return
	}
	gDir, err = filepath.Abs(gDir)
	if err != nil {
		handlerFatalErr(err)
		return
	}
	err = os.Chdir(gDir)
	if err != nil {
		handlerFatalErr(err)
		return
	}

	err = watch()
	if err != nil {
		handlerFatalErr(err)
		return
	}
}

func parseArgs() {
	flag.StringVar(&gDir, "d", "", "The directory to be watch")

	flag.Parse()
}

func watch() error {
	var err error
	gWatcher, err = fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer gWatcher.Close()

	dirList := list.New()
	getRecursiveDirList(gDir, dirList)

	for e := dirList.Front(); e != nil; e = e.Next() {

		fmt.Println(e.Value)

		err = gWatcher.Add(e.Value.(string))
		if err != nil {
			return err
		}
	}

loop:
	for {
		select {
		case event := <-gWatcher.Events:
			if event.Op&fsnotify.Write != fsnotify.Write {
				continue loop
			}

			if filepath.Ext(event.Name) != ".go" {
				continue loop
			}

			now := time.Now()
			if gPeriod.Add(time.Second).After(now) {
				continue loop
			}
			gPeriod = now

			fmt.Printf("\n>> File [%s] has changed! Rerun the program!\n", event.Name)
			restart()

		case err := <-gWatcher.Errors:
			fmt.Println("Watcher error:", err)
		}
	}

	return nil
}

func getRecursiveDirList(dir string, dirList *list.List) error {
	dirList.PushBack(dir)

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
		err = getRecursiveDirList(filepath.Join(dir, fInfo.Name()), dirList)
		if err != nil {
			return err
		}
	}

	return nil
}

func stop() {
	if gCmd != nil && gCmd.Process != nil {
		err := gCmd.Process.Kill()
		if err != nil {
			fmt.Println("KILL ERROR:", err)
		}
	}
}

func start() {
	gCmd = exec.Command("go", "run", "main.go")
	gCmd.Stdin = os.Stdin
	gCmd.Stdout = os.Stdout
	gCmd.Stderr = os.Stderr
	gCmd.Env = os.Environ()
	go gCmd.Run()
}

func restart() {
	stop()
	start()
}

func handlerFatalErr(err error) {
	usage := fmt.Sprintf("\nUsage: %s [OPTIONS]\n", NAME)
	fmt.Fprintln(os.Stderr, err)
	fmt.Fprintln(os.Stderr, usage)
	flag.PrintDefaults()
}

const (
	NAME    = "hotrun"
	VERSION = "0.1"
)

var (
	gDir string
)

var (
	gWatcher *fsnotify.Watcher
	gCmd     *exec.Cmd

	gPeriod time.Time = time.Now()
)

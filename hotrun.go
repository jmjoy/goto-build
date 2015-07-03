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

	autoRun()

	err = watch()
	if err != nil {
		handlerFatalErr(err)
		return
	}
}

func parseArgs() {
	flag.StringVar(&gDir, "d", "", "The directory to be watch")
	flag.StringVar(&gOuput, "o", "", "The Oput filename")

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
			autoBuild()
			gChanRestart <- true

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

func autoBuild() {
	var output string
	if gOuput != "" {
		output = gOuput
	} else {
		output = filepath.Base(gDir)
	}

	cmd := exec.Command("go", "build", "-o", output)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	cmd.Run()
}

func autoRun() {
	var cmdName string
	if gOuput != "" {
		cmdName = gOuput
	} else {
		cmdName = filepath.Base(gDir)
	}
	cmd := exec.Command("./" + cmdName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	go func() {
		err := cmd.Run()
		if err != nil {
			fmt.Println("RUN ERROR:", err)
		}
	}()

	go func() {
		<-gChanRestart
		if cmd != nil && cmd.Process != nil {
			err := cmd.Process.Kill()
			if err != nil {
				fmt.Println("KILL ERROR:", err)
			}
		}
		_ = cmd
		autoRun()
	}()
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
	gDir   string
	gOuput string
)

var (
	gWatcher *fsnotify.Watcher

	gPeriod      = time.Now()
	gChanRestart = make(chan bool)
)

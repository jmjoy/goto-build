package main

import (
	"container/list"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

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
		err = getRecursiveDirList(fInfo.Name(), dirList)
		if err != nil {
			return err
		}
	}

	return nil
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

var gWatcher *fsnotify.Watcher

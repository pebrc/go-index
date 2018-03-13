package main

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"
	"strconv"
	"github.com/fsnotify/fsnotify"
)

func targetPath(target string, date time.Time, file string) string {
	return filepath.Join(target, strconv.Itoa(date.Year()), strconv.Itoa(int(date.Month())), filepath.Base(file))
}

func same(src, target string) bool {
	info1, err1 := os.Stat(src)
	info2, err2 := os.Stat(target)
	return err1 == nil && err2 == nil && os.SameFile(info1, info2)
}

func link(src, target string) {
	if same(src, target) {
		log.Println("already indexed")
		return;
	}
		
	path := filepath.Dir(target)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, os.ModeDir | (04 << 6 | 02 << 6 | 01 << 6))
		if err != nil {
			log.Fatal("could not create parent dir: ", err)
		}
	}
	if _, err := os.Stat(target); err == nil {
		log.Println("index entry seems to exist. deleting: ", target)
		err := os.Remove(target)
		if err != nil {
			log.Fatal("could not remove existing index entry", err)
		}
	}
	
	err := os.Symlink(src, target)
	if err != nil {
		log.Fatal("could not create symlink between ", src, target, err)
	}
}

func parseWithFallback(dateStr string) (time.Time, error) {
	date, err := time.Parse("20060102", dateStr)
	if err != nil {
		return time.Parse("02012006", dateStr)
	}
	return date, err

}


func newWatcher(target, src string, done chan bool)(*fsnotify.Watcher) {

	r, err := regexp.Compile("([0-9]{8})[^0-9]+")
	if err != nil {
		log.Fatal("could not compile regex", err)
	}
	
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
		
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				log.Println("event:", event)
				match := r.FindStringSubmatch(event.Name)
				if len(match) > 0 {
					date, err := parseWithFallback(match[1])
					log.Println("affected file matches date regex", date, err)
					indexPath := targetPath(target, date, event.Name)
					if event.Op&(fsnotify.Create | fsnotify.Chmod|fsnotify.Write) != 0 {
						log.Println("linking ", event)
						link(event.Name, indexPath)
					}
					if event.Op&fsnotify.Remove == fsnotify.Remove {
						err := os.Remove(indexPath)
						if err != nil {
							log.Println("could not delete", indexPath, err)
						}
					}

				} else {
					log.Println("no date match ... ignoring")
				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
				done <- true
			}
		}
	}()

	err = watcher.Add(src)
	if err != nil {
		log.Fatal(err)
	}
	return watcher
}


func main() {
	argsWithoutProgram := os.Args[1:]
	if len(argsWithoutProgram) < 2 {
		log.Fatal("at least one index target and one source directory need to specified: cmd target src+ ")
	}

	done := make(chan bool)
	target := argsWithoutProgram[0]
	watchers := make([]*fsnotify.Watcher, len(os.Args[2:]))
	for idx, src := range os.Args[2:]{
		log.Println(strconv.Itoa(idx) +  " starting to watch:", src)
		watchers[idx] = newWatcher(target, src, done)
	}

	<- done
	for _, watcher := range watchers {
		watcher.Close()
	}
			
}

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/charithe/timedbuf"
	"github.com/fsnotify/fsnotify"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	webhookUrl        string
	notificationFlush = 100
	notificationDelay = 1 * time.Second
)

//noinspection GoSnakeCaseUsage
type DiscordMessage struct {
	Username string `json:"username"`
	Content  string `json:"content"`
}

func sendDiscord(jsonData DiscordMessage) {
	jsonBuf := new(bytes.Buffer)
	errj := json.NewEncoder(jsonBuf).Encode(jsonData)
	if errj != nil {
		logger.Printf("error: invalid json: %v", errj)
	}
	resp, errp := http.Post(webhookUrl, "application/json; charset=utf-8", jsonBuf)
	if errp != nil {
		logger.Printf("error: cannot send discord notification: %v", errp)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		logger.Printf("error: discord response: %v", resp)
	}
	_ = resp.Body.Close()
}

func notify(items []interface{}) {
	var ss []string
	for _, item := range items {
		if path, err := filepath.Rel(RootFolder, fmt.Sprint(item)); err == nil {
			ss = append(ss, PublicUrl+filepath.ToSlash(path))
		}
	}
	jsonData := DiscordMessage{
		Username: "Gallery",
		Content:  "See what was just published: \n" + strings.Join(ss[:], "\n"),
	}
	go sendDiscord(jsonData)
}

func startFsWatcher(path string, webhook string) {
	if webhook == "" { // No WebHook no cry
		return
	}
	webhookUrl = webhook

	var err error
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = watcher.Close() }()

	logger.Printf("Monitoring for filesystem changes with delay %v", notificationDelay)
	eventBuffer := timedbuf.New(notificationFlush, notificationDelay, notify)
	defer eventBuffer.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				//logger.Print("event:", event)
				if event.Op&fsnotify.Create == fsnotify.Create {
					newStat, _ := os.Stat(event.Name)
					if newStat.Size() == 0 { // We want bytes, not just names
						break
					}
					eventBuffer.Put(event.Name)
					if newStat.IsDir() {
						go func() { // Watch in new folders, but wait first
							time.Sleep(2 * time.Second)
							_ = watcher.Add(event.Name)
							logger.Printf("Watching for new files in %v", event.Name)
						}()
					}
				}
				// CRASHES:
				//if event.Op&fsnotify.Remove == fsnotify.Remove {
				//	newStat, _ := os.Stat(event.Name)
				//	if newStat.IsDir() { // Stop watching the removed folders
				//		_ = watcher.Remove(event.Name)
				//	}
				//}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Print("error:", err)
			}
		}
	}()

	err = watcher.Add(path)
	if err != nil {
		log.Fatal(err)
	}
	<-done
}

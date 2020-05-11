package gallery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"foldergal/config"
	"github.com/charithe/timedbuf"
	"github.com/fsnotify/fsnotify"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	watcher        *fsnotify.Watcher
	notifChanSize  = 100
	WatchedFolders = 0
	logger         = &config.Global.Log
)

type discordMessage struct {
	Username string `json:"username"`
	Content  string `json:"content"`
}

func sendDiscord(jsonData discordMessage) {
	jsonBuf := new(bytes.Buffer)
	errj := json.NewEncoder(jsonBuf).Encode(jsonData)
	if errj != nil {
		(*logger).Printf("error: invalid json: %v", errj)
	}
	resp, errp := http.Post(config.Global.DiscordWebhook,
		"application/json; charset=utf-8", jsonBuf)
	if errp != nil {
		(*logger).Printf("error: cannot send discord notification: %v", errp)
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		(*logger).Printf("error: discord response: %v", resp)
	}
}

func notify(items []interface{}) {
	uniqueUrls := make(map[string]int)
	for i, item := range items {
		if path, err := filepath.Rel(config.Global.Root, fmt.Sprint(item)); err == nil {
			dirPath := filepath.Dir(path)
			var urlPage *url.URL
			if dirPath == "." {
				urlPage, _ = url.Parse(config.Global.PublicUrl)
			} else {
				urlPage, _ = url.Parse(config.Global.PublicUrl + filepath.ToSlash(dirPath) + "?by-date")
			}
			uniqueUrls[urlPage.String()] = i
		}
	}
	urlStrings := make([]string, 0, len(uniqueUrls))
	for k := range uniqueUrls {
		urlStrings = append(urlStrings, k)
	}
	if len(urlStrings) == 0 {
		return
	}
	jsonData := discordMessage{
		Username: config.Global.DiscordName,
		Content:  "See what was just published: \n" + strings.Join(urlStrings[:], "\n"),
	}
	sendDiscord(jsonData)
}

// Watch every folder in config.Global.Root and send notification on new file
func StartFsWatcher() {
	var err error
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		(*logger).Print(err)
		return
	}
	defer func() { _ = watcher.Close() }()
	notifyDuration := time.Duration(config.Global.NotifyAfter)
	(*logger).Printf("Monitoring for filesystem changes with delay %v", notifyDuration)
	eventBuffer := timedbuf.New(notifChanSize, notifyDuration, notify)
	defer eventBuffer.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Create == fsnotify.Create {
					eventBuffer.Put(event.Name)
					newStat, _ := os.Stat(event.Name)
					if newStat.IsDir() {
						_ = watcher.Add(event.Name)
						(*logger).Printf("Watching for new files in %v", event.Name)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				(*logger).Print("error:", err)
			}
		}
	}()

	err = filepath.Walk(config.Global.Root,
		func(walkPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				return nil
			}
			err = watcher.Add(walkPath)
			if err != nil {
				return err
			}
			WatchedFolders++
			return nil
		})
	if err != nil {
		(*logger).Print(err)
	}

	err = watcher.Add(config.Global.Root)
	if err != nil {
		(*logger).Print(err)
		return
	}
	WatchedFolders++
	<-done
}

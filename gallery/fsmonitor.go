package gallery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"foldergal/config"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/charithe/timedbuf"
	"github.com/fsnotify/fsnotify"
)

var (
	watcher        *fsnotify.Watcher
	notifChanSize  = 100
	WatchedFolders = 0
	logger         = &config.Global.Log
)

type discordImage struct {
	Url string `json:"url"`
}
type discordEmbed struct {
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Url         string       `json:"url"`
	Image       discordImage `json:"image"`
}

type discordMessage struct {
	Username string         `json:"username"`
	Content  string         `json:"content"`
	Embeds   []discordEmbed `json:"embeds"`
}

func sendDiscord(jsonData discordMessage) {

	jsonBuf := new(bytes.Buffer)
	errj := json.NewEncoder(jsonBuf).Encode(jsonData)
	if errj != nil {
		(*logger).Printf("error: invalid json: %v", errj)
	}
	resp, errp := http.Post(config.Global.DiscordWebhook,
		"application/json; charset=utf-8", jsonBuf)
	if resp != nil {
		defer resp.Body.Close() // Prevent leaks?!
	}
	if errp != nil {
		(*logger).Printf("error: cannot send discord notification: %v", errp)
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		(*logger).Printf("error: discord response: %v", resp)
	}
}

var slashes = regexp.MustCompile(`/`)

func notify(items []interface{}) {
	jsonData := discordMessage{
		Username: config.Global.DiscordName,
		Embeds:   []discordEmbed{},
	}
	uniqueEmbeds := make(map[string]discordEmbed)

	for _, item := range items {
		sItem := fmt.Sprint(item)
		if filepath.Ext(sItem) == "" || !IsValidMedia(sItem) {
			// Ignore non files and non media files
			continue
		}
		if path, err := filepath.Rel(config.Global.Root, sItem); err == nil {
			uniqueEmbeds[path] = discordEmbed{
				Title: filepath.Base(sItem),
				Description: slashes.ReplaceAllString(filepath.ToSlash(
					filepath.Dir(path)), " • "),
				Url: config.Global.PublicUrl +
					EscapePath(filepath.ToSlash(filepath.Dir(path))) +
					"#" + EscapePath(filepath.Base(sItem)),
				Image: discordImage{Url: config.Global.PublicUrl +
					EscapePath(filepath.ToSlash(path)) + "?thumb"},
			}
		}
	}
	embeds := make([]discordEmbed, 0, len(uniqueEmbeds))
	for _, embed := range uniqueEmbeds {
		embeds = append(embeds, embed)
	}
	totalEmbeds := len(embeds)
	if totalEmbeds == 0 {
		return
	}
	maxEmbeds := 10
	if totalEmbeds > maxEmbeds {
		// Split to multiple messages
		// because the discord api docs says
		// only 10 embeds are allowed per message
		for i := 0; i < totalEmbeds; i += maxEmbeds {
			bound := i + maxEmbeds
			if bound > totalEmbeds {
				bound = totalEmbeds
			}
			jsonData.Content = "New media in " + config.Global.PublicUrl
			jsonData.Embeds = embeds[i:bound]
			sendDiscord(jsonData)
		}
	} else {
		jsonData.Content = "New media in " + config.Global.PublicUrl
		jsonData.Embeds = embeds
		sendDiscord(jsonData)
	}
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

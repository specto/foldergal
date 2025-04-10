package templates

import (
	"fmt"
	htmlTpl "html/template"
	"io"
	textTpl "text/template"
	"time"

	"specto.org/projects/foldergal/internal/config"
	"specto.org/projects/foldergal/internal/storage"
)

type BreadCrumb struct {
	Url   string
	Title string
}

// Base page
type Page struct {
	Title        string
	Prefix       string
	AppVersion   string
	AppBuildTime string
	LinkPrev     string
	LinkNext     string
	ParentUrl    string
	ParentName   string
	ShowOverlay  bool
}

type ListItem struct {
	ModTime time.Time
	Id      string
	Url     string
	Name    string
	Thumb   string
	Class   string
	W       int
	H       int
}

// Page used for folder list
type List struct {
	Items       []ListItem
	BreadCrumbs []BreadCrumb
	Page
	Description    string
	Copyright      string
	ParentUrl      string
	LinkPrev       string
	LinkNext       string
	LinkOrderAsc   string
	LinkOrderDesc  string
	LinkSortName   string
	LinkSortDate   string
	ItemCount      string
	DisplayMode    string
	IsSortedByName bool
	IsReversed     bool
}

type ErrorPage struct {
	Page
	Message string
}

type TwoColTable struct {
	Page
	Rows [][2]string
}

type FeedItem struct {
	Title string
	Type  string
	Url   string
	Thumb string
	Id    string
	Date  string
	Mdate time.Time
}

type FeedPage struct {
	Items     []FeedItem
	FeedUrl   string
	SiteTitle string
	SiteUrl   string
	LastDate  string
}

type ViewPage struct {
	Page
	MediaPath string
}

var (
	Feed *textTpl.Template
	Html *htmlTpl.Template
)

func formatDate(date time.Time) string {
	return date.In(config.Global.TimeLocation).Format("2006-01-02 15:04 Z07")
}

func parseHtmlTemplates(templs ...string) (t *htmlTpl.Template, err error) {
	t = htmlTpl.New("_all")

	for i, templ := range templs {
		listFile, errF := storage.Internal.Open(templ)
		if errF != nil {
			panic(errF)
		}
		listBytes, _ := io.ReadAll(listFile)
		if _, err = t.New(fmt.Sprint("_", i)).Funcs(
			htmlTpl.FuncMap{"formatDate": formatDate},
		).Parse(string(listBytes)); err != nil {
			return
		}
		_ = listFile.Close()
	}

	return
}

func parseTextTemplates(templs ...string) (t *textTpl.Template, err error) {
	t = textTpl.New("_all")

	for i, templ := range templs {
		listFile, errF := storage.Internal.Open(templ)
		if errF != nil {
			panic(errF)
		}
		listBytes, _ := io.ReadAll(listFile)
		if _, err = t.New(
			fmt.Sprint("_", i)).Parse(string(listBytes)); err != nil {
			return
		}
		_ = listFile.Close()
	}

	return
}

func init() {
	var err error
	Html, err = parseHtmlTemplates(
		"res/templates/list.html",
		"res/templates/footer.html",
		"res/templates/error.html",
		"res/templates/layout.html",
		"res/templates/view.html",
		"res/templates/table.html",
	)
	if err != nil {
		panic(err)
	}

	Feed, err = parseTextTemplates("res/feed.xml")
	if err != nil {
		panic(err)
	}
}

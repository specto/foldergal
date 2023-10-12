package templates

import (
	"fmt"
	htmlTpl "html/template"
	"io"
	"specto.org/projects/foldergal/internal/config"
	"specto.org/projects/foldergal/internal/storage"
	textTpl "text/template"
	"time"
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
	ShowOverlay  bool
}

type ListItem struct {
	Id      string
	Url     string
	Name    string
	Thumb   string
	ModTime time.Time
	Class   string
	W       int
	H       int
}

// Page used for folder list
type List struct {
	ParentUrl   string
	ItemCount   string
	SortedBy    string
	IsReversed  bool
	DisplayMode string
	Items       []ListItem
	BreadCrumbs []BreadCrumb
	Page
}

type ErrorPage struct {
	Page
	Message string
}

type TwoColTable struct {
	Page
	Rows [][2]string
}

type RssItem struct {
	Title string
	Type  string
	Url   string
	Thumb string
	Id    string
	Mdate time.Time
	Date  string
}

type RssPage struct {
	FeedUrl   string
	SiteTitle string
	SiteUrl   string
	LastDate  string
	Items     []RssItem
}

var (
	Rss  *textTpl.Template
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
		"res/templates/table.html",
	)
	if err != nil {
		panic(err)
	}
	Rss, err = parseTextTemplates("res/rss.xml")

	if err != nil {
		panic(err)
	}
}

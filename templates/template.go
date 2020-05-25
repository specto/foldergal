package templates

import (
	"fmt"
	"foldergal/storage"
	"html/template"
	"io/ioutil"
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
}

type ListItem struct {
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
	Slideshow	string
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
	Html *template.Template
)

func parseTemplates(templs ...string) (t *template.Template, err error) {
	t = template.New("_all")

	for i, templ := range templs {
		listFile, errF := storage.Internal.Open(templ)
		if errF != nil {
			panic(errF)
		}
		listBytes, _ := ioutil.ReadAll(listFile)
		if _, err = t.New(fmt.Sprint("_", i)).Parse(string(listBytes)); err != nil {
			return
		}
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
		listBytes, _ := ioutil.ReadAll(listFile)
		if _, err = t.New(fmt.Sprint("_", i)).Parse(
			string(listBytes)); err != nil {
			return
		}
	}

	return
}

func init() {
	var err error
	Html, err = parseTemplates(
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

package templates

import (
	"fmt"
	"foldergal/embedded"
	"html/template"
	"io/ioutil"
	"time"
)

type BreadCrumb struct {
	Url   string
	Title string
}

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

type List struct {
	ParentUrl   string
	ItemCount   string
	SortedBy    string
	Items       []ListItem
	BreadCrumbs []BreadCrumb
	Page
}

type ErrorPage struct {
	Page
	Message string
}

var Tpl *template.Template

func parseTemplates(templs ...string) (t *template.Template, err error) {
	t = template.New("_all")

	for i, templ := range templs {
		listFile, errF := embedded.Fs.Open(templ)
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

func Initialize() {
	var err error
	Tpl, err = parseTemplates(
		"res/templates/list.html",
		"res/templates/footer.html",
		"res/templates/error.html",
		"res/templates/layout.html",
	)
	if err != nil {
		panic(err)
	}
}

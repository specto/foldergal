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
	ItemCount    string
	BreadCrumbs  []BreadCrumb
	SortedBy     string
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
	ParentUrl string
	Items     []ListItem
	Page
}

var ListTpl *template.Template

func parseTemplates(templs ...string) (t *template.Template, err error) {
	t = template.New("_all")

	for i, templ := range templs {
		if _, err = t.New(fmt.Sprint("_", i)).Parse(templ); err != nil {
			return
		}
	}

	return
}
func init() {
	var err error

	listFile, _ := embedded.Fs.Open("res/templates/list.html")
	listBytes, _ := ioutil.ReadAll(listFile)

	footFile, _ := embedded.Fs.Open("res/templates/footer.html")
	footBytes, _ := ioutil.ReadAll(footFile)
	ListTpl, err = parseTemplates(
		string(listBytes),
		string(footBytes),
	)
	if err != nil {
		panic(err)
	}
}

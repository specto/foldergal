package templates

import "html/template"

type BreadCrumb struct {
	Url   string
	Title string
}

type Page struct {
	Title        string
	Prefix       string
	AppVersion   string
	AppBuildTime string
	BreadCrumbs  []BreadCrumb
}

type ListItem struct {
	Url   string
	Name  string
	Thumb string
	W     int
	H     int
}

type List struct {
	ParentUrl string
	Items     []ListItem
	Page
}

var ListTpl = template.Must(template.New("").ParseFiles(
	"src/templates/layout.gohtml",
	"src/templates/list.gohtml",
	"src/templates/footer.gohtml",
))

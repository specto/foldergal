package templates

import "html/template"

type Page struct {
	Title  string
	Prefix string
}

type ListItem struct {
	Url   string
	Name  string
	Thumb string
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

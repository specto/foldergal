package main

import (
	"flag"
	"fmt"
	"github.com/kardianos/osext"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
)

type Page struct {
	Title string
	Body  []byte
}

func (p *Page) save() error {
	filename := p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func loadPage(title string) (*Page, error) {
	filename := title + ".txt"
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, _ *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}


var _TEMPLATES *template.Template

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := _TEMPLATES.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var _VALIDPATH = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := _VALIDPATH.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func main() {
	folderPath, err := osext.ExecutableFolder()
	if err != nil {
		log.Fatal(err)
	}

	host := flag.String("host", "localhost", "host address to bind to")
	port := flag.Int("port", 8080, "port to run at")
	home := flag.String("home", folderPath, "home folder")
	flag.Parse()

	log.Printf("Home folder is: %v", home)

	_TEMPLATES = template.Must(template.ParseGlob(*home + "/templates/*"))

	httpmux := http.NewServeMux()
	httpmux.HandleFunc("/view/", makeHandler(viewHandler))
	httpmux.HandleFunc("/edit/", makeHandler(editHandler))
	httpmux.HandleFunc("/save/", makeHandler(saveHandler))

	bind:= fmt.Sprintf("%s:%d", *host, *port)
	log.Printf("Running at %v", bind)
	log.Fatal(http.ListenAndServe(bind, httpmux))
}

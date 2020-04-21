package main

import (
	"./templates"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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



func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ListHtml.ExecuteTemplate(w, "T", p)
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

func getEnvWithDefault(key string, defaultValue string) string {
	if envValue := os.Getenv(key); envValue != "" {
		return envValue
	} else {
		return defaultValue
	}
}

var Logger *log.Logger

func main() {

	defaultHost := getEnvWithDefault("FOLDERGAL_HOST", "localhost")
	defaultPort, err := strconv.Atoi(getEnvWithDefault("FOLDERGAL_PORT", "8080"))
	if err != nil {
		log.Fatal(err)
	}
	defaultHome := getEnvWithDefault("FOLDERGAL_HOME", "")
	if defaultHome == "" {
		ex, _ := os.Executable()
		defaultHome = filepath.Dir(ex)
	}
	host := flag.String("host", defaultHost, "host address to bind to")
	port := flag.Int("port", defaultPort, "port to run at")
	home := flag.String("home", defaultHome, "home folder")
	flag.Parse()

	logFile, err := os.OpenFile(filepath.Join(*home, "foldergal.log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Print("Error: Log file cannot be created in home directory.")
		log.Fatal(err)
	}
	defer logFile.Close()

	Logger = log.New(logFile, "foldergal: ", log.Lshortfile | log.LstdFlags)

	//Logger.Printf("Env is: %v", os.Environ())
	Logger.Printf("Home folder is: %v", *home)

	httpmux := http.NewServeMux()
	httpmux.HandleFunc("/view/", makeHandler(viewHandler))
	httpmux.HandleFunc("/edit/", makeHandler(editHandler))
	httpmux.HandleFunc("/save/", makeHandler(saveHandler))

	bind:= fmt.Sprintf("%s:%d", *host, *port)
	Logger.Printf("Running at %v", bind)

	srvErr := http.ListenAndServe(bind, httpmux)
	defer log.Fatal(srvErr)
}

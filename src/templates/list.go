package templates

import (
	"fmt"
	"html/template"
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
func init () {
	var err error
	ListTpl, err = parseTemplates(
		`{{ define "layout" }}
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <link rel="shortcut icon" href="{{ .Prefix }}static?favicon" />
    <title>{{ if .Title -}}
        {{- .Title -}}
    {{- else -}}
            Foldergal
        {{- end }}</title>
    {{template "head" .}}
</head>
<body>
    {{template "body" .}}
    {{template "footer" .}}
</body>
</html>
{{end}}`,
		`{{ define "head" }}<link rel="stylesheet" type="text/css" href="{{ .Prefix }}static?css" />{{end}}`,
		`{{ define "body" }}
    <header>
        <nav>
            <h1>
                {{ range .BreadCrumbs -}}
                    <a href="{{ .Url }}" title="{{ .Title }}">{{ .Title }}</a>
                {{- end }}
                <span>&gt;</span>
            </h1>
        </nav>
    </header>
    <main>
    <ul>
        {{ if .ParentUrl }}
            <li>
                <a class="title-container folder" href="{{- .ParentUrl -}}">
                    <span><img src="{{ .Prefix }}static?up" alt="{{ .ParentUrl }}" title="{{ .ParentUrl }}" /></span>
                    <span class="title clear">..</span>
                </a></li>
        {{ end }}
        {{ range .Items }}
            <li><a class="title-container" href="{{- .Url -}}" title="{{ .Name }}">
                <span>
                    {{ if .Thumb -}}
                        <img src="{{ .Thumb }}" alt="{{ .Name }}" />
                    {{- end }}

                    <span class="title">{{- .Name -}}</span>
                </span></a></li>
        {{ end }}
    </ul>
    </main>
{{ end }}`,
		`{{define "footer"}}
<footer>
    foldergal v:{{ .AppVersion }}
</footer>
{{end}}`,
	)
	if err != nil {
		panic(err)
	}
}
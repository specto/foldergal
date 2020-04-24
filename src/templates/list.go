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
    <link rel="shortcut icon" href="/?favicon" />
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
		`{{ define "head" }}
<style type="text/css">
    body
    {
        font-family: sans-serif;
        color: black;
        background: #EDEDED;
    }

    a
    {
        text-decoration: none;
        border-radius: 0.5em;
    }

    a.media { background: white; }
    a:hover { background-color: #D6D6D6; }

    a:active
    {
        background-color: #D1DDF0;
        color: darkred;
    }

    body > header a
    {
        color: black;
        display: inline-block;
        border-radius: 0;
        border-bottom-width: 2px;
        border-bottom-style: dotted;
        border-bottom-color: transparent;
        padding-right: 0.2em;
		max-width: 18em;
		overflow: hidden;
		white-space: nowrap; 
 		text-overflow: ellipsis;
    }
	body > header span {
		overflow: hidden;
		display: inline-block;
		white-space: nowrap;
	}

    body > header a:hover
    {
        border-bottom-color: black;
        background: none;
    }

    body > header a:active { border-bottom-color: darkred; }

    body > header a::after
    {
        content: ' \005C';
        display: inline-block;
        padding-left: 0.2em;
    }

    body > header a:last-child { padding: 0; }
    body > header a:first-child::after,
    body > header a:last-child::after { content: ''; }
    body > header a:only-child::after { padding: 0; }

    body > footer
    {
        margin: 2em 1em;
        padding-top: 0.5em;
        color: gray;
        font-size: 0.8em;
        text-align: right;
    }

    body > footer::before
    {
        content: '…(˶‾᷄ ⁻̫ ‾᷅˵)…';
        display: block;
    }

    main ul
    {
        display: flex;
        flex-wrap: wrap;
        align-items: flex-start;
        flex-direction: row;
        padding: 0;
        margin: 0;
    }

    main li
    {
        display: flex;
        padding: 0.25em;
        margin: 0;
        list-style: none;
        align-items: center;
    }

    main a
    {
        width: 10em;
        min-height: 7em;
        display: flex;
        padding: 0.5em;
        flex-direction: column;
        justify-content: center;
        align-items: center;
        color: black;
    }

    main a span
    {
        display: flex;
        flex-grow: 1;
        overflow-wrap: break-word;
        word-break: break-all;
        text-align: center;
        align-items: center;
        justify-content: center;
    }

    main a img
    {
        height: 8em;
        width: 9em;
        object-fit: contain;
        object-position: center bottom;
        display: inline-block;
    }

    main .author-container, main .title-container { position: relative; }

    main .author,
    main .title
    {
        position: absolute;
        right: 0;
        bottom: 0;
        display: inline-block;
        color: white;
        font-size: 0.8em;
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
    }

    main .author
    {
        padding-right: 0.4em;
        width: 3.1em;
        text-align: right;
        z-index: 10;
    }

    main .title
    {
        left: 0;
        padding-right: 0.4em;
        padding-left: 0.4em;
        width: 13em;
        background: rgba(0, 0, 0, 0.6);
        border-radius: 0 0 0.5em 0.5em;
        text-align: left;
        z-index: 5;
    }

    main .author-container .title
    {
        width: 10.4em;
        padding-right: 3em;
    }

    main .folder .title
    {
        background: none;
        color: black;
        text-align: center;
        padding: 0.4em 0 0.4em 0;
        width: 13.5em;
    }

    main .big a
    {
        width: 18em;
        min-height: 14em;
    }

    main .big a img
    {
        max-height: 14em;
        max-width: 16em;
    }
</style>
{{end}}
{{ define "body" }}
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
    {{ $prefix := .Prefix }}
    <ul>
        {{ if .ParentUrl }}
            <li>
                <a class="title-container folder" href="{{- .ParentUrl -}}">
                    <span><img src="/go?up" alt="{{ .ParentUrl }}" title="{{ .ParentUrl }}" /></span>
                    <span class="title clear">..</span>
                </a></li>
        {{ end }}
        {{ range .Items }}
            <li><a class="title-container" href="{{- $prefix -}}{{- .Url -}}" title="{{ .Name }}">
                <span>
                    {{ if .Thumb -}}
                        <img src="{{- $prefix -}}{{ .Thumb }}" alt="{{ .Name }}" />
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
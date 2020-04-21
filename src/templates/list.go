package templates

import "html/template"

var ListHtml, _ = template.New("foo").Parse(`{{define "T"}}<!DOCTYPE html>
<html>
<h1>List</h1>
<main>
{{.}}
</main>
</html>{{end}}`)
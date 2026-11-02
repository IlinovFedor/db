WHERE {{quote .Col}} {{if or (eq .Op "IS NULL") (eq .Op "IS NOT NULL")}}{{.Op}}{{else}}{{.Op}} {{quoteLit .Val}}{{end}}

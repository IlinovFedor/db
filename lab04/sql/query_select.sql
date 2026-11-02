SELECT {{.SelectList}} FROM {{quote .Table}}{{if .WhereClause}} {{.WhereClause}}{{end}}{{if .OrderClause}} {{.OrderClause}}{{end}};

DELETE FROM {{quote .Table}}{{if .WhereClause}} {{.WhereClause}}{{end}};

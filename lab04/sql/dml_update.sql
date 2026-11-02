UPDATE {{quote .Table}} SET {{quote .SetCol}} = {{quoteLit .SetVal}}{{if .WhereClause}} {{.WhereClause}}{{end}};

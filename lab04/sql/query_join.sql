SELECT {{.SelectList}}
FROM {{quote .Table1}} AS t1
INNER JOIN {{quote .Table2}} AS t2
    ON t1.{{quote .Key1}} = t2.{{quote .Key2}};

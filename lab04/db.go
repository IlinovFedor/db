package main

import (
	"embed"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/jackc/pgx"
)

//go:embed sql/*.sql
var sqlFS embed.FS

type SQLManager struct {
	templates *template.Template
	rawSQL    map[string]string
}

func NewSQLManager() (*SQLManager, error) {
	mgr := &SQLManager{
		rawSQL: make(map[string]string),
	}

	funcMap := template.FuncMap{
		"quote":    quoteIdent,
		"quoteLit": quoteLiteral,
	}

	tmpl := template.New("sql").Funcs(funcMap)

	entries, err := sqlFS.ReadDir("sql")
	if err != nil {
		return nil, fmt.Errorf("read sql dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		data, err := sqlFS.ReadFile("sql/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read file %s: %w", entry.Name(), err)
		}
		content := string(data)
		mgr.rawSQL[entry.Name()] = content

		t := tmpl.New(entry.Name())
		if _, err := t.Parse(content); err != nil {
			return nil, fmt.Errorf("parse sql template %s: %w", entry.Name(), err)
		}
	}

	mgr.templates = tmpl
	return mgr, nil
}

func (m *SQLManager) Get(name string) string {
	return m.rawSQL[name]
}

func (m *SQLManager) Render(name string, data any) (string, error) {
	t := m.templates.Lookup(name)
	if t == nil {
		return "", fmt.Errorf("template %s not found", name)
	}
	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.String(), nil
}

func (m *SQLManager) CreateTable(tableName string, colDefs []string) (string, error) {
	return m.Render("ddl_create_table.sql", map[string]any{
		"TableName":         tableName,
		"ColumnDefinitions": strings.Join(colDefs, ",\n    "),
	})
}

func (m *SQLManager) ColumnDef(name, colType string, isNullable bool) string {
	res, _ := m.Render("ddl_column_def.sql", map[string]any{
		"Name":       name,
		"Type":       colType,
		"IsNullable": isNullable,
	})
	return res
}

func (m *SQLManager) PrimaryKey(col string) string {
	res, _ := m.Render("ddl_primary_key.sql", map[string]any{
		"Col": col,
	})
	return res
}

func (m *SQLManager) ForeignKey(col, foreignTable, foreignCol, onDel, onUpd string) string {
	if onDel == "NO ACTION" {
		onDel = ""
	}
	if onUpd == "NO ACTION" {
		onUpd = ""
	}
	res, _ := m.Render("ddl_foreign_key.sql", map[string]any{
		"Col":          col,
		"ForeignTable": foreignTable,
		"ForeignCol":   foreignCol,
		"OnDelete":     onDel,
		"OnUpdate":     onUpd,
	})
	return res
}

func (m *SQLManager) AlterAdd(table, col, colType string) (string, error) {
	return m.Render("ddl_alter_add.sql", map[string]any{
		"Table":  table,
		"Column": col,
		"Type":   colType,
	})
}

func (m *SQLManager) AlterType(table, col, colType string) (string, error) {
	return m.Render("ddl_alter_type.sql", map[string]any{
		"Table":  table,
		"Column": col,
		"Type":   colType,
	})
}

func (m *SQLManager) AlterDrop(table, col string) (string, error) {
	return m.Render("ddl_alter_drop.sql", map[string]any{
		"Table":  table,
		"Column": col,
	})
}

func (m *SQLManager) TruncateTable(table string) (string, error) {
	return m.Render("ddl_truncate.sql", map[string]any{
		"Table": table,
	})
}

func (m *SQLManager) DropTable(table string) (string, error) {
	return m.Render("ddl_drop.sql", map[string]any{
		"Table": table,
	})
}

func (m *SQLManager) DropTableIfExists(table string) (string, error) {
	return m.Render("drop_table_if_exists.sql", map[string]any{
		"Table": table,
	})
}

func (m *SQLManager) Insert(table string, quotedCols, rowTuples []string) (string, error) {
	return m.Render("dml_insert.sql", map[string]any{
		"Table":   table,
		"Columns": strings.Join(quotedCols, ", "),
		"Values":  strings.Join(rowTuples, ",\n"),
	})
}

func (m *SQLManager) Update(table, setCol, setVal, whereClause string) (string, error) {
	return m.Render("dml_update.sql", map[string]any{
		"Table":       table,
		"SetCol":      setCol,
		"SetVal":      setVal,
		"WhereClause": strings.TrimSpace(whereClause),
	})
}

func (m *SQLManager) Delete(table, whereClause string) (string, error) {
	return m.Render("dml_delete.sql", map[string]any{
		"Table":       table,
		"WhereClause": strings.TrimSpace(whereClause),
	})
}

func (m *SQLManager) SimpleWhere(col, op, val string) string {
	if col == "" || op == "" {
		return ""
	}
	res, err := m.Render("where_simple.sql", map[string]any{
		"Col": col,
		"Op":  op,
		"Val": val,
	})
	if err != nil {
		return ""
	}
	return " " + strings.TrimSpace(res)
}

func (m *SQLManager) Select(table, selectList, whereClause, orderClause string) (string, error) {
	return m.Render("query_select.sql", map[string]any{
		"Table":       table,
		"SelectList":  selectList,
		"WhereClause": whereClause,
		"OrderClause": orderClause,
	})
}

func (m *SQLManager) Join(t1, t2, col1, col2 string, selectItems []string) (string, error) {
	return m.Render("query_join.sql", map[string]any{
		"Table1":     t1,
		"Table2":     t2,
		"Key1":       col1,
		"Key2":       col2,
		"SelectList": strings.Join(selectItems, ", "),
	})
}

func (m *SQLManager) SelectAs(tableAlias, col, resultCol string) string {
	res, _ := m.Render("query_select_as.sql", map[string]any{
		"TableAlias": tableAlias,
		"Col":        col,
		"ResultCol":  resultCol,
	})
	return res
}

func (m *SQLManager) WhereNull(col, op string) string {
	res, _ := m.Render("query_where_null.sql", map[string]any{
		"Col": col,
		"Op":  op,
	})
	return res
}

func (m *SQLManager) WhereIn(col, op string, items []string) string {
	res, _ := m.Render("query_where_in.sql", map[string]any{
		"Col":    col,
		"Op":     op,
		"Values": strings.Join(items, ", "),
	})
	return res
}

func (m *SQLManager) WhereOp(col, op, val string) string {
	res, _ := m.Render("query_where_op.sql", map[string]any{
		"Col": col,
		"Op":  op,
		"Val": val,
	})
	return res
}

func (m *SQLManager) WhereClause(cond string) string {
	res, _ := m.Render("query_where_clause.sql", map[string]any{
		"Condition": cond,
	})
	return res
}

func (m *SQLManager) OrderBy(col, dir string) string {
	res, _ := m.Render("query_order_by.sql", map[string]any{
		"Col": col,
		"Dir": dir,
	})
	return res
}

func (m *SQLManager) TableDataQuery(table string) (string, error) {
	return m.Render("query_table_data.sql", map[string]any{
		"Table": table,
	})
}

type ColumnInfo struct {
	Name     string
	DataType string
}

type TableData struct {
	Name    string
	Columns []string
	Rows    [][]any
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func quoteLiteral(val string) string {
	return `'` + strings.ReplaceAll(val, `'`, `''`) + `'`
}

func initDatabaseTables(pool *pgx.ConnPool, sql *SQLManager) error {
	if _, err := pool.Exec(sql.Get("schema.sql")); err != nil {
		return fmt.Errorf("failed to execute schema SQL: %w", err)
	}

	var deptCount int
	if err := pool.QueryRow(sql.Get("count_departments.sql")).Scan(&deptCount); err != nil {
		return err
	}

	if deptCount == 0 {
		if _, err := pool.Exec(sql.Get("seed_departments.sql")); err != nil {
			return fmt.Errorf("failed to seed departments: %w", err)
		}
	}

	var empCount int
	if err := pool.QueryRow(sql.Get("count_employees.sql")).Scan(&empCount); err != nil {
		return err
	}

	if empCount == 0 {
		if _, err := pool.Exec(sql.Get("seed_employees.sql")); err != nil {
			return fmt.Errorf("failed to seed employees: %w", err)
		}
	}

	return nil
}

func resetDatabaseTables(pool *pgx.ConnPool, sql *SQLManager) error {
	tables, err := getTables(pool, sql)
	if err != nil {
		return err
	}
	for _, t := range tables {
		dropSQL, err := sql.DropTableIfExists(t)
		if err != nil {
			return err
		}
		_, _ = pool.Exec(dropSQL)
	}
	return initDatabaseTables(pool, sql)
}

func getTables(pool *pgx.ConnPool, sql *SQLManager) ([]string, error) {
	rows, err := pool.Query(sql.Get("get_tables.sql"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func getColumns(pool *pgx.ConnPool, sql *SQLManager, table string) ([]ColumnInfo, error) {
	rows, err := pool.Query(sql.Get("get_columns.sql"), table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		if err := rows.Scan(&col.Name, &col.DataType); err != nil {
			return nil, err
		}
		cols = append(cols, col)
	}
	return cols, rows.Err()
}

func scanRowsToTableData(rows *pgx.Rows, tableName string) (*TableData, error) {
	fields := rows.FieldDescriptions()
	colNames := make([]string, len(fields))
	for i, f := range fields {
		colNames[i] = f.Name
	}

	var dataRows [][]any
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		for i, v := range vals {
			if t, ok := v.(time.Time); ok {
				vals[i] = t.Format("2006-01-02")
			}
		}
		dataRows = append(dataRows, vals)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &TableData{
		Name:    tableName,
		Columns: colNames,
		Rows:    dataRows,
	}, nil
}

func getTableData(pool *pgx.ConnPool, sql *SQLManager, table string) (*TableData, error) {
	sqlQuery, err := sql.TableDataQuery(table)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(sqlQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToTableData(rows, table)
}

func getAllTablesData(pool *pgx.ConnPool, sql *SQLManager) ([]TableData, error) {
	tables, err := getTables(pool, sql)
	if err != nil {
		return nil, err
	}
	var res []TableData
	for _, t := range tables {
		td, err := getTableData(pool, sql, t)
		if err != nil {
			continue
		}
		res = append(res, *td)
	}
	return res, nil
}

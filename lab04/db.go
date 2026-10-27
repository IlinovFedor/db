package main

import (
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx"
)

//go:embed sql/schema.sql
var schemaSQL string

//go:embed sql/seed_employees.sql
var seedEmployeesSQL string

//go:embed sql/seed_departments.sql
var seedDepartmentsSQL string

//go:embed sql/get_tables.sql
var getTablesSQL string

//go:embed sql/get_columns.sql
var getColumnsSQL string

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

func initDatabaseTables(pool *pgx.ConnPool) error {
	if _, err := pool.Exec(schemaSQL); err != nil {
		return fmt.Errorf("failed to execute schema SQL: %w", err)
	}

	var deptCount int
	if err := pool.QueryRow("SELECT COUNT(*) FROM departments").Scan(&deptCount); err != nil {
		return err
	}

	if deptCount == 0 {
		if _, err := pool.Exec(seedDepartmentsSQL); err != nil {
			return fmt.Errorf("failed to seed departments: %w", err)
		}
	}

	var empCount int
	if err := pool.QueryRow("SELECT COUNT(*) FROM lab2_employee_ilinov").Scan(&empCount); err != nil {
		return err
	}

	if empCount == 0 {
		if _, err := pool.Exec(seedEmployeesSQL); err != nil {
			return fmt.Errorf("failed to seed employees: %w", err)
		}
	}

	return nil
}

func resetDatabaseTables(pool *pgx.ConnPool) error {
	tables, err := getTables(pool)
	if err != nil {
		return err
	}
	for _, t := range tables {
		_, _ = pool.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", quoteIdent(t)))
	}
	return initDatabaseTables(pool)
}

func getTables(pool *pgx.ConnPool) ([]string, error) {
	rows, err := pool.Query(getTablesSQL)
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

func getColumns(pool *pgx.ConnPool, table string) ([]ColumnInfo, error) {
	rows, err := pool.Query(getColumnsSQL, table)
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

func buildSimpleWhere(col, op, val string) string {
	if col == "" || op == "" {
		return ""
	}
	if op == "IS NULL" || op == "IS NOT NULL" {
		return fmt.Sprintf(" WHERE %s %s", quoteIdent(col), op)
	}
	return fmt.Sprintf(" WHERE %s %s %s", quoteIdent(col), op, quoteLiteral(val))
}

func getTableData(pool *pgx.ConnPool, table string) (*TableData, error) {
	sqlQuery := fmt.Sprintf("SELECT * FROM %s;", quoteIdent(table))
	rows, err := pool.Query(sqlQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRowsToTableData(rows, table)
}

func getAllTablesData(pool *pgx.ConnPool) ([]TableData, error) {
	tables, err := getTables(pool)
	if err != nil {
		return nil, err
	}
	var res []TableData
	for _, t := range tables {
		td, err := getTableData(pool, t)
		if err != nil {
			continue
		}
		res = append(res, *td)
	}
	return res, nil
}

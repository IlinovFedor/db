package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx"
)

func renderDMLError(w http.ResponseWriter, views *ViewRenderer, pool *pgx.ConnPool, op string, step int, table, msg, querySQL string, extra func(*DMLPageData)) error {
	base, err := newBasePage(pool)
	if err != nil {
		return err
	}
	base.Status = "error"
	base.Message = msg
	base.SQL = querySQL
	base.SelectedTable = table

	var tableCols []ColumnInfo
	if table != "" {
		tableCols, _ = getColumns(pool, table)
	}

	data := DMLPageData{
		BasePage:     base,
		DMLOp:        op,
		DMLStep:      step,
		TableColumns: tableCols,
	}
	if extra != nil {
		extra(&data)
	}
	return views.RenderStatus(w, "dml", http.StatusBadRequest, data)
}

func parseInsertRows(r *http.Request, cols []string, rowCount int) ([]map[string]string, []string) {
	insertRows := make([]map[string]string, rowCount)
	var rowTuples []string

	for rIdx := 0; rIdx < rowCount; rIdx++ {
		rowMap := make(map[string]string, len(cols))
		var valLiterals []string
		for _, c := range cols {
			val := r.FormValue(fmt.Sprintf("val_%d_%s", rIdx, c))
			rowMap[c] = val
			if val == "" {
				valLiterals = append(valLiterals, "NULL")
			} else {
				valLiterals = append(valLiterals, quoteLiteral(val))
			}
		}
		insertRows[rIdx] = rowMap
		rowTuples = append(rowTuples, "("+strings.Join(valLiterals, ", ")+")")
	}
	return insertRows, rowTuples
}

func HandleDML(views *ViewRenderer, pool *pgx.ConnPool) AppHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		base, err := newBasePage(pool)
		if err != nil {
			return &AppError{Err: err, Message: "Database error: " + err.Error(), Code: http.StatusInternalServerError}
		}

		op := r.PathValue("op")
		if op == "" {
			op = r.URL.Query().Get("op")
		}
		if op == "" {
			op = "insert"
		}

		step := 1
		if r.URL.Query().Get("step") == "2" {
			step = 2
		}

		table := r.URL.Query().Get("table")
		if table == "" && len(base.Tables) > 0 {
			table = base.Tables[0]
		}

		var tableCols []ColumnInfo
		if table != "" {
			tableCols, _ = getColumns(pool, table)
		}

		rowCount, _ := strconv.Atoi(r.URL.Query().Get("rows"))
		if rowCount <= 0 {
			rowCount = 1
		}

		base.SelectedTable = table
		return views.Render(w, "dml", DMLPageData{
			BasePage:     base,
			DMLOp:        op,
			DMLStep:      step,
			DMLRowCount:  rowCount,
			TableColumns: tableCols,
		})
	}
}

func InsertHandler(views *ViewRenderer, pool *pgx.ConnPool) AppHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := r.ParseForm(); err != nil {
			return &AppError{Err: err, Message: "Invalid form data", Code: http.StatusBadRequest}
		}

		table := strings.TrimSpace(r.FormValue("table"))
		rowCount, _ := strconv.Atoi(r.FormValue("row_count"))
		if rowCount <= 0 {
			rowCount = 1
		}

		if table == "" {
			return renderDMLError(w, views, pool, "insert", 2, "", "Select a table", "", nil)
		}

		cols, err := getColumns(pool, table)
		if err != nil {
			return renderDMLError(w, views, pool, "insert", 2, table, "Failed to get columns: "+err.Error(), "", nil)
		}

		var targetCols []string
		for _, col := range cols {
			targetCols = append(targetCols, col.Name)
		}

		var quotedCols []string
		for _, c := range targetCols {
			quotedCols = append(quotedCols, quoteIdent(c))
		}

		insertRows, rowTuples := parseInsertRows(r, targetCols, rowCount)

		renderErr := func(msg, sqlQuery string) error {
			return renderDMLError(w, views, pool, "insert", 2, table, msg, sqlQuery, func(d *DMLPageData) {
				d.DMLRowCount = rowCount
				d.InsertRows = insertRows
			})
		}

		querySQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES\n%s;",
			quoteIdent(table),
			strings.Join(quotedCols, ", "),
			strings.Join(rowTuples, ",\n"))

		if _, err := pool.Exec(querySQL); err != nil {
			return renderErr(fmt.Sprintf("Failed to execute INSERT INTO: %v", err), querySQL)
		}

		redirectSuccess(w, r, fmt.Sprintf("/tables/%s", url.PathEscape(table)))
		return nil
	}
}

func UpdateHandler(views *ViewRenderer, pool *pgx.ConnPool) AppHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := r.ParseForm(); err != nil {
			return &AppError{Err: err, Message: "Invalid form data", Code: http.StatusBadRequest}
		}

		table := strings.TrimSpace(r.FormValue("table"))
		setCol := strings.TrimSpace(r.FormValue("set_col"))
		setVal := strings.TrimSpace(r.FormValue("set_val"))
		whereCol := strings.TrimSpace(r.FormValue("where_col"))
		whereOp := strings.TrimSpace(r.FormValue("where_op"))
		whereVal := strings.TrimSpace(r.FormValue("where_val"))

		renderErr := func(msg, sqlQuery string) error {
			return renderDMLError(w, views, pool, "update", 1, table, msg, sqlQuery, func(d *DMLPageData) {
				d.UpdateSetCol = setCol
				d.UpdateSetVal = setVal
				d.UpdateWhereCol = whereCol
				d.UpdateWhereOp = whereOp
				d.UpdateWhereVal = whereVal
			})
		}

		if table == "" || setCol == "" {
			return renderErr("Specify table and column to update", "")
		}

		querySQL := fmt.Sprintf("UPDATE %s SET %s = %s%s;",
			quoteIdent(table),
			quoteIdent(setCol),
			quoteLiteral(setVal),
			buildSimpleWhere(whereCol, whereOp, whereVal))

		if _, err := pool.Exec(querySQL); err != nil {
			return renderErr(fmt.Sprintf("Failed to execute UPDATE: %v", err), querySQL)
		}

		redirectSuccess(w, r, fmt.Sprintf("/tables/%s", url.PathEscape(table)))
		return nil
	}
}

func DeleteHandler(views *ViewRenderer, pool *pgx.ConnPool) AppHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := r.ParseForm(); err != nil {
			return &AppError{Err: err, Message: "Invalid form data", Code: http.StatusBadRequest}
		}

		table := strings.TrimSpace(r.FormValue("table"))
		whereCol := strings.TrimSpace(r.FormValue("where_col"))
		whereOp := strings.TrimSpace(r.FormValue("where_op"))
		whereVal := strings.TrimSpace(r.FormValue("where_val"))

		renderErr := func(msg, sqlQuery string) error {
			return renderDMLError(w, views, pool, "delete", 1, table, msg, sqlQuery, func(d *DMLPageData) {
				d.DeleteWhereCol = whereCol
				d.DeleteWhereOp = whereOp
				d.DeleteWhereVal = whereVal
			})
		}

		if table == "" {
			return renderErr("Select a table", "")
		}

		querySQL := fmt.Sprintf("DELETE FROM %s%s;", quoteIdent(table), buildSimpleWhere(whereCol, whereOp, whereVal))

		if _, err := pool.Exec(querySQL); err != nil {
			return renderErr(fmt.Sprintf("Failed to execute DELETE: %v", err), querySQL)
		}

		redirectSuccess(w, r, fmt.Sprintf("/tables/%s", url.PathEscape(table)))
		return nil
	}
}

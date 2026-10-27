package main

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/jackc/pgx"
)

func HandleSelect(views *ViewRenderer, pool *pgx.ConnPool) AppHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		base, err := newBasePage(pool)
		if err != nil {
			return &AppError{Err: err, Message: "Database error: " + err.Error(), Code: http.StatusInternalServerError}
		}

		targetTable := r.PathValue("table")
		if targetTable == "" {
			targetTable = r.URL.Query().Get("table")
		}
		if targetTable == "" && len(base.Tables) > 0 {
			targetTable = base.Tables[0]
		}

		var cols []ColumnInfo
		if targetTable != "" {
			cols, _ = getColumns(pool, targetTable)
		}

		base.SelectedTable = targetTable
		return views.Render(w, "select", SelectPageData{
			BasePage:      base,
			SelectTable:   targetTable,
			SelectColumns: cols,
		})
	}
}

func SelectHandler(views *ViewRenderer, pool *pgx.ConnPool) AppHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := r.ParseForm(); err != nil {
			return &AppError{Err: err, Message: "Invalid form data", Code: http.StatusBadRequest}
		}

		table := strings.TrimSpace(r.FormValue("table"))
		cols, _ := getColumns(pool, table)

		base, err := newBasePage(pool)
		if err != nil {
			return &AppError{Err: err, Message: "Database error: " + err.Error(), Code: http.StatusInternalServerError}
		}

		data := SelectPageData{
			BasePage:      base,
			SelectTable:   table,
			SelectColumns: cols,
		}

		var projCols []string
		for _, col := range cols {
			if r.FormValue("inc_"+col.Name) == "1" {
				projCols = append(projCols, quoteIdent(col.Name))
			}
		}

		selectList := "*"
		if len(projCols) > 0 {
			selectList = strings.Join(projCols, ", ")
		}

		var whereParts []string
		for _, col := range cols {
			if r.FormValue("where_en_"+col.Name) == "1" {
				op := strings.TrimSpace(r.FormValue("where_op_" + col.Name))
				rawVal := strings.TrimSpace(r.FormValue("where_val_" + col.Name))

				switch op {
				case "IS NULL", "IS NOT NULL":
					whereParts = append(whereParts, fmt.Sprintf("%s %s", quoteIdent(col.Name), op))
				case "IN", "NOT IN":
					items := strings.Split(rawVal, ",")
					var quotedItems []string
					for _, it := range items {
						if trimmed := strings.TrimSpace(it); trimmed != "" {
							quotedItems = append(quotedItems, quoteLiteral(trimmed))
						}
					}
					if len(quotedItems) > 0 {
						whereParts = append(whereParts, fmt.Sprintf("%s %s (%s)", quoteIdent(col.Name), op, strings.Join(quotedItems, ", ")))
					}
				case "LIKE", "ILIKE":
					whereParts = append(whereParts, fmt.Sprintf("%s %s %s", quoteIdent(col.Name), op, quoteLiteral("%"+rawVal+"%")))
				default:
					whereParts = append(whereParts, fmt.Sprintf("%s %s %s", quoteIdent(col.Name), op, quoteLiteral(rawVal)))
				}
			}
		}

		whereClause := ""
		if len(whereParts) > 0 {
			whereClause = " WHERE " + strings.Join(whereParts, " AND ")
		}

		orderByCol := strings.TrimSpace(r.FormValue("order_by"))
		orderDir := strings.TrimSpace(r.FormValue("order_dir"))
		if orderDir != "DESC" {
			orderDir = "ASC"
		}
		orderClause := ""
		if orderByCol != "" && orderByCol != "none" {
			orderClause = fmt.Sprintf(" ORDER BY %s %s", quoteIdent(orderByCol), orderDir)
		}

		querySQL := fmt.Sprintf("SELECT %s FROM %s%s%s;", selectList, quoteIdent(table), whereClause, orderClause)

		rows, err := pool.Query(querySQL)
		if err != nil {
			data.Status = "error"
			data.Message = "Failed to execute SELECT: " + err.Error()
			data.SQL = querySQL
			return views.Render(w, "select", data)
		}
		defer rows.Close()

		tableData, err := scanRowsToTableData(rows, table)
		if err != nil {
			data.Status = "error"
			data.Message = "Failed to read query results: " + err.Error()
			data.SQL = querySQL
			return views.Render(w, "select", data)
		}

		data.QueryResult = tableData
		return views.Render(w, "select", data)
	}
}

func HandleJoin(views *ViewRenderer, pool *pgx.ConnPool) AppHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		base, err := newBasePage(pool)
		if err != nil {
			return &AppError{Err: err, Message: "Database error: " + err.Error(), Code: http.StatusInternalServerError}
		}

		step := 1
		if r.URL.Query().Get("step") == "2" {
			step = 2
		}

		t1 := r.URL.Query().Get("t1")
		t2 := r.URL.Query().Get("t2")
		if t1 == "" && len(base.Tables) > 0 {
			t1 = "lab2_employee_ilinov"
			if !slices.Contains(base.Tables, t1) {
				t1 = base.Tables[0]
			}
		}
		if t2 == "" && len(base.Tables) > 1 {
			t2 = "departments"
			if !slices.Contains(base.Tables, t2) {
				t2 = base.Tables[1]
			}
		}

		var cols1, cols2 []ColumnInfo
		if t1 != "" {
			cols1, _ = getColumns(pool, t1)
		}
		if t2 != "" {
			cols2, _ = getColumns(pool, t2)
		}

		return views.Render(w, "join", JoinPageData{
			BasePage:     base,
			JoinStep:     step,
			JoinTable1:   t1,
			JoinTable2:   t2,
			JoinColumns1: cols1,
			JoinColumns2: cols2,
		})
	}
}

func JoinHandler(views *ViewRenderer, pool *pgx.ConnPool) AppHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := r.ParseForm(); err != nil {
			return &AppError{Err: err, Message: "Invalid form data", Code: http.StatusBadRequest}
		}

		t1 := strings.TrimSpace(r.FormValue("table1"))
		t2 := strings.TrimSpace(r.FormValue("table2"))
		col1 := strings.TrimSpace(r.FormValue("join_col1"))
		col2 := strings.TrimSpace(r.FormValue("join_col2"))

		cols1, _ := getColumns(pool, t1)
		cols2, _ := getColumns(pool, t2)

		base, err := newBasePage(pool)
		if err != nil {
			return &AppError{Err: err, Message: "Database error: " + err.Error(), Code: http.StatusInternalServerError}
		}

		data := JoinPageData{
			BasePage:     base,
			JoinStep:     2,
			JoinTable1:   t1,
			JoinTable2:   t2,
			JoinColumns1: cols1,
			JoinColumns2: cols2,
		}

		if t1 == "" || t2 == "" || col1 == "" || col2 == "" {
			data.Status = "error"
			data.Message = "Specify both tables and join columns for ON condition"
			return views.Render(w, "join", data)
		}

		var selectItems []string
		for _, c := range cols1 {
			if r.FormValue("sel1_"+c.Name) == "1" {
				selectItems = append(selectItems, fmt.Sprintf("t1.%s AS %s", quoteIdent(c.Name), quoteIdent(t1+"."+c.Name)))
			}
		}
		for _, c := range cols2 {
			if r.FormValue("sel2_"+c.Name) == "1" {
				selectItems = append(selectItems, fmt.Sprintf("t2.%s AS %s", quoteIdent(c.Name), quoteIdent(t2+"."+c.Name)))
			}
		}
		if len(selectItems) == 0 {
			selectItems = append(selectItems, "t1.*", "t2.*")
		}

		querySQL := fmt.Sprintf("SELECT %s\nFROM %s AS t1\nINNER JOIN %s AS t2\n    ON t1.%s = t2.%s;",
			strings.Join(selectItems, ", "),
			quoteIdent(t1),
			quoteIdent(t2),
			quoteIdent(col1),
			quoteIdent(col2))

		rows, err := pool.Query(querySQL)
		if err != nil {
			data.Status = "error"
			data.Message = "Failed to execute INNER JOIN: " + err.Error()
			data.SQL = querySQL
			return views.Render(w, "join", data)
		}
		defer rows.Close()

		tableData, err := scanRowsToTableData(rows, fmt.Sprintf("%s ⨝ %s", t1, t2))
		if err != nil {
			data.Status = "error"
			data.Message = "Failed to read query results: " + err.Error()
			data.SQL = querySQL
			return views.Render(w, "join", data)
		}

		data.JoinResult = tableData
		return views.Render(w, "join", data)
	}
}

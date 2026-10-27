package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx"
)

func renderDDLError(w http.ResponseWriter, views *ViewRenderer, pool *pgx.ConnPool, op string, step int, selectedTable, msg, querySQL string, extra func(*DDLPageData)) error {
	base, err := newBasePage(pool)
	if err != nil {
		return err
	}
	base.Status = "error"
	base.Message = msg
	base.SQL = querySQL
	base.SelectedTable = selectedTable

	data := DDLPageData{
		BasePage: base,
		DDLOp:    op,
		DDLStep:  step,
	}
	if extra != nil {
		extra(&data)
	}
	return views.RenderStatus(w, "ddl", http.StatusBadRequest, data)
}

func parseCreateColumns(r *http.Request, colCount int, tableName string) ([]ColumnDef, []string) {
	pkIndex, pkErr := strconv.Atoi(r.FormValue("pk_index"))
	hasPK := pkErr == nil && pkIndex >= 0 && pkIndex < colCount

	parsedCols := make([]ColumnDef, colCount)
	var colDefs []string
	var pkColName string

	for i := 0; i < colCount; i++ {
		cName := strings.TrimSpace(r.FormValue(fmt.Sprintf("col_name_%d", i)))
		cType := strings.TrimSpace(r.FormValue(fmt.Sprintf("col_type_%d", i)))
		if cType == "" {
			cType = "TEXT"
		}
		isNullable := r.FormValue(fmt.Sprintf("col_nullable_%d", i)) == "1"
		fkTable := strings.TrimSpace(r.FormValue(fmt.Sprintf("fk_table_%d", i)))
		fkColRaw := strings.TrimSpace(r.FormValue(fmt.Sprintf("fk_col_%d", i)))
		fkSelfIdx := -1

		if strings.HasPrefix(fkColRaw, "__self__:") {
			fkTable = "__self__"
			if idx, err := strconv.Atoi(strings.TrimPrefix(fkColRaw, "__self__:")); err == nil {
				fkSelfIdx = idx
			}
		} else if strings.Contains(fkColRaw, ":") {
			parts := strings.SplitN(fkColRaw, ":", 2)
			fkTable = parts[0]
			fkColRaw = parts[1]
		} else if fkSelfStr := strings.TrimSpace(r.FormValue(fmt.Sprintf("fk_self_col_%d", i))); fkSelfStr != "" {
			if idx, err := strconv.Atoi(fkSelfStr); err == nil {
				fkSelfIdx = idx
			}
		}

		if fkTable == "__self__" && fkSelfIdx == -1 {
			if idx, err := strconv.Atoi(fkColRaw); err == nil {
				fkSelfIdx = idx
			}
		}

		isPK := hasPK && i == pkIndex
		parsedCols[i] = ColumnDef{
			Name:         cName,
			DataType:     cType,
			IsNullable:   isNullable,
			IsPrimaryKey: isPK,
			FKTable:      fkTable,
			FKCol:        fkColRaw,
			FKSelfIndex:  fkSelfIdx,
			FKOnDelete:   strings.TrimSpace(r.FormValue(fmt.Sprintf("fk_on_delete_%d", i))),
			FKOnUpdate:   strings.TrimSpace(r.FormValue(fmt.Sprintf("fk_on_update_%d", i))),
		}

		if cName == "" {
			continue
		}

		colDef := fmt.Sprintf("%s %s", quoteIdent(cName), cType)
		if !isNullable {
			colDef += " NOT NULL"
		}
		colDefs = append(colDefs, colDef)

		if isPK {
			pkColName = cName
		}
	}

	if pkColName != "" {
		colDefs = append(colDefs, fmt.Sprintf("PRIMARY KEY (%s)", quoteIdent(pkColName)))
	}

	for _, c := range parsedCols {
		if c.Name == "" {
			continue
		}
		targetTable := c.FKTable
		targetCol := c.FKCol
		if targetTable == "__self__" {
			targetTable = tableName
			if c.FKSelfIndex >= 0 && c.FKSelfIndex < colCount {
				targetCol = parsedCols[c.FKSelfIndex].Name
			}
		}

		if targetTable != "" && targetCol != "" {
			fkRule := fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s (%s)",
				quoteIdent(c.Name), quoteIdent(targetTable), quoteIdent(targetCol))
			if c.FKOnDelete != "" && c.FKOnDelete != "NO ACTION" {
				fkRule += " ON DELETE " + c.FKOnDelete
			}
			if c.FKOnUpdate != "" && c.FKOnUpdate != "NO ACTION" {
				fkRule += " ON UPDATE " + c.FKOnUpdate
			}
			colDefs = append(colDefs, fkRule)
		}
	}

	return parsedCols, colDefs
}

func HandleDDL(views *ViewRenderer, pool *pgx.ConnPool) AppHandler {
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
			op = "create"
		}

		step := 1
		if r.URL.Query().Get("step") == "2" {
			step = 2
		}

		colCount, _ := strconv.Atoi(r.URL.Query().Get("count"))
		if colCount <= 0 {
			colCount = 2
		}

		targetTable := r.URL.Query().Get("table")
		if targetTable == "" && len(base.Tables) > 0 {
			targetTable = base.Tables[0]
		}

		var tableCols []ColumnInfo
		if targetTable != "" {
			tableCols, _ = getColumns(pool, targetTable)
		}

		alterAction := r.URL.Query().Get("action")
		if alterAction == "" {
			alterAction = "alter_type"
		}

		allTableCols := make(map[string][]string)
		var createCols []ColumnDef
		if op == "create" && step == 2 {
			createCols = make([]ColumnDef, colCount)
			for i := range createCols {
				createCols[i] = ColumnDef{
					Name:         fmt.Sprintf("column_%d", i+1),
					DataType:     "TEXT",
					IsNullable:   true,
					IsPrimaryKey: i == 0,
					FKSelfIndex:  -1,
				}
			}
			allTableCols = getAllTableColumns(pool, base.Tables)
		}

		base.SelectedTable = targetTable
		return views.Render(w, "ddl", DDLPageData{
			BasePage:        base,
			DDLOp:           op,
			DDLStep:         step,
			DDLColCount:     colCount,
			CreateColumns:   createCols,
			AlterAction:     alterAction,
			TableColumns:    tableCols,
			AllTableColumns: allTableCols,
		})
	}
}

func CreateTableHandler(views *ViewRenderer, pool *pgx.ConnPool) AppHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := r.ParseForm(); err != nil {
			return &AppError{Err: err, Message: "Invalid form data", Code: http.StatusBadRequest}
		}

		tableName := strings.TrimSpace(r.FormValue("table_name"))
		colCount, _ := strconv.Atoi(r.FormValue("col_count"))
		if colCount <= 0 {
			colCount = 1
		}

		parsedCols, colDefs := parseCreateColumns(r, colCount, tableName)

		renderErr := func(msg, sqlQuery string) error {
			return renderDDLError(w, views, pool, "create", 2, "", msg, sqlQuery, func(d *DDLPageData) {
				d.DDLColCount = colCount
				d.TableName = tableName
				d.CreateColumns = parsedCols
				d.AllTableColumns = getAllTableColumns(pool, d.Tables)
			})
		}

		if tableName == "" {
			return renderErr("Table name cannot be empty", "")
		}
		if len(colDefs) == 0 {
			return renderErr("At least one column must be specified", "")
		}

		querySQL := fmt.Sprintf("CREATE TABLE %s (\n    %s\n);", quoteIdent(tableName), strings.Join(colDefs, ",\n    "))
		if _, err := pool.Exec(querySQL); err != nil {
			return renderErr(fmt.Sprintf("Failed to create table (CREATE TABLE): %v", err), querySQL)
		}

		redirectSuccess(w, r, fmt.Sprintf("/tables/%s", url.PathEscape(tableName)))
		return nil
	}
}

func AlterTableHandler(views *ViewRenderer, pool *pgx.ConnPool) AppHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := r.ParseForm(); err != nil {
			return &AppError{Err: err, Message: "Invalid form data", Code: http.StatusBadRequest}
		}

		table := strings.TrimSpace(r.FormValue("table"))
		action := strings.TrimSpace(r.FormValue("alter_action"))
		colName := strings.TrimSpace(r.FormValue("col_name"))
		newCol := strings.TrimSpace(r.FormValue("new_col_name"))
		effectiveColName := colName
		if action == "add" && newCol != "" {
			effectiveColName = newCol
		}
		colType := strings.TrimSpace(r.FormValue("col_type"))

		renderErr := func(msg, sqlQuery string) error {
			return renderDDLError(w, views, pool, "alter", 1, table, msg, sqlQuery, func(d *DDLPageData) {
				d.AlterAction = action
				d.AlterColName = colName
				d.AlterNewColName = newCol
				d.AlterColType = colType
				if table != "" {
					d.TableColumns, _ = getColumns(pool, table)
				}
			})
		}

		if table == "" || effectiveColName == "" {
			return renderErr("Specify table and column name", "")
		}

		var querySQL string
		switch action {
		case "add":
			if colType == "" {
				colType = "TEXT"
			}
			querySQL = fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", quoteIdent(table), quoteIdent(effectiveColName), colType)
		case "alter_type":
			if colType == "" {
				colType = "TEXT"
			}
			querySQL = fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s USING %s::%s;", quoteIdent(table), quoteIdent(effectiveColName), colType, quoteIdent(effectiveColName), colType)
		case "drop":
			querySQL = fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s CASCADE;", quoteIdent(table), quoteIdent(effectiveColName))
		default:
			return renderErr("Unknown ALTER action", "")
		}

		if _, err := pool.Exec(querySQL); err != nil {
			return renderErr(fmt.Sprintf("Failed to execute ALTER TABLE: %v", err), querySQL)
		}

		redirectSuccess(w, r, fmt.Sprintf("/tables/%s", url.PathEscape(table)))
		return nil
	}
}

func TruncateTableHandler(views *ViewRenderer, pool *pgx.ConnPool) AppHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := r.ParseForm(); err != nil {
			return &AppError{Err: err, Message: "Invalid form data", Code: http.StatusBadRequest}
		}

		table := strings.TrimSpace(r.FormValue("table"))
		if table == "" {
			return renderDDLError(w, views, pool, "truncate", 1, "", "Select a table to truncate", "", nil)
		}

		querySQL := fmt.Sprintf("TRUNCATE TABLE %s CASCADE;", quoteIdent(table))
		if _, err := pool.Exec(querySQL); err != nil {
			return renderDDLError(w, views, pool, "truncate", 1, table, fmt.Sprintf("Failed to execute TRUNCATE: %v", err), querySQL, nil)
		}

		redirectSuccess(w, r, fmt.Sprintf("/tables/%s", url.PathEscape(table)))
		return nil
	}
}

func DropTableHandler(views *ViewRenderer, pool *pgx.ConnPool) AppHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := r.ParseForm(); err != nil {
			return &AppError{Err: err, Message: "Invalid form data", Code: http.StatusBadRequest}
		}

		table := strings.TrimSpace(r.FormValue("table"))
		if table == "" {
			return renderDDLError(w, views, pool, "drop", 1, "", "Select a table to drop", "", nil)
		}

		querySQL := fmt.Sprintf("DROP TABLE %s CASCADE;", quoteIdent(table))
		if _, err := pool.Exec(querySQL); err != nil {
			return renderDDLError(w, views, pool, "drop", 1, table, fmt.Sprintf("Failed to execute DROP TABLE: %v", err), querySQL, nil)
		}

		redirectSuccess(w, r, "/tables")
		return nil
	}
}

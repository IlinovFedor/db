package main

import (
	"net/http"

	"github.com/jackc/pgx"
)

func HandleTables(views *ViewRenderer, pool *pgx.ConnPool) AppHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		base, err := newBasePage(pool)
		if err != nil {
			return &AppError{Err: err, Message: "Database error: " + err.Error(), Code: http.StatusInternalServerError}
		}

		data := TablesPageData{BasePage: base}
		tableName := r.PathValue("name")
		if tableName == "" {
			tableName = r.URL.Query().Get("table")
		}

		if tableName == "" {
			data.ShowViewAll = true
			if all, err := getAllTablesData(pool); err == nil {
				data.AllTables = all
			}
		} else {
			data.SelectedTable = tableName
			td, err := getTableData(pool, tableName)
			if err != nil {
				data.Status = "error"
				data.Message = "Table not found: " + err.Error()
				return views.RenderStatus(w, "tables", http.StatusNotFound, data)
			}
			data.CurrentTable = td
		}

		return views.Render(w, "tables", data)
	}
}

func ResetHandler(views *ViewRenderer, pool *pgx.ConnPool) AppHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := resetDatabaseTables(pool); err != nil {
			base, bErr := newBasePage(pool)
			if bErr == nil {
				base.Status = "error"
				base.Message = "Database reset failed: " + err.Error()
				return views.RenderStatus(w, "tables", http.StatusInternalServerError, TablesPageData{BasePage: base, ShowViewAll: true})
			}
			return &AppError{Err: err, Message: "Database reset failed: " + err.Error(), Code: http.StatusInternalServerError}
		}
		redirectSuccess(w, r, "/tables")
		return nil
	}
}

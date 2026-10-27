package main

import (
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/jackc/pgx"
)

type ViewRenderer struct {
	templates map[string]*template.Template
}

func NewViewRenderer(funcMap template.FuncMap) (*ViewRenderer, error) {
	layout, err := template.New("layout.html").Funcs(funcMap).ParseFiles("templates/layout.html")
	if err != nil {
		return nil, fmt.Errorf("parse layout.html: %w", err)
	}

	pages := []string{"tables", "ddl", "dml", "select", "join"}
	tmpls := make(map[string]*template.Template, len(pages))
	for _, p := range pages {
		t, err := layout.Clone()
		if err != nil {
			return nil, fmt.Errorf("clone layout for %s: %w", p, err)
		}
		path := filepath.Join("templates", p+".html")
		t, err = t.ParseFiles(path)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		tmpls[p] = t
	}
	return &ViewRenderer{templates: tmpls}, nil
}

func (v *ViewRenderer) Render(w http.ResponseWriter, name string, data any) error {
	return v.RenderStatus(w, name, http.StatusOK, data)
}

func (v *ViewRenderer) RenderStatus(w http.ResponseWriter, name string, status int, data any) error {
	tmpl, ok := v.templates[name]
	if !ok {
		return fmt.Errorf("template %q not found", name)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	return tmpl.ExecuteTemplate(w, "layout.html", data)
}

type BasePage struct {
	Tables        []string
	SelectedTable string
	Status        string
	Message       string
	SQL           string
}

type TablesPageData struct {
	BasePage
	ShowViewAll  bool
	CurrentTable *TableData
	AllTables    []TableData
}

type ColumnDef struct {
	Name         string
	DataType     string
	IsNullable   bool
	IsPrimaryKey bool
	FKTable      string
	FKCol        string
	FKSelfIndex  int
	FKOnDelete   string
	FKOnUpdate   string
}

type DDLPageData struct {
	BasePage
	DDLOp           string
	DDLStep         int
	DDLColCount     int
	TableName       string
	CreateColumns   []ColumnDef
	AlterAction     string
	AlterColName    string
	AlterNewColName string
	AlterColType    string
	TableColumns    []ColumnInfo
	AllTableColumns map[string][]string
}

type DMLPageData struct {
	BasePage
	DMLOp          string
	DMLStep        int
	DMLRowCount    int
	InsertRows     []map[string]string
	UpdateSetCol   string
	UpdateSetVal   string
	UpdateWhereCol string
	UpdateWhereOp  string
	UpdateWhereVal string
	DeleteWhereCol string
	DeleteWhereOp  string
	DeleteWhereVal string
	TableColumns   []ColumnInfo
}

type SelectPageData struct {
	BasePage
	SelectTable   string
	SelectColumns []ColumnInfo
	QueryResult   *TableData
}

type JoinPageData struct {
	BasePage
	JoinStep     int
	JoinTable1   string
	JoinTable2   string
	JoinColumns1 []ColumnInfo
	JoinColumns2 []ColumnInfo
	JoinResult   *TableData
}

type AppError struct {
	Err     error
	Message string
	Code    int
}

func (e *AppError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "Internal Server Error"
}

func (e *AppError) Unwrap() error {
	return e.Err
}

type AppHandler func(w http.ResponseWriter, r *http.Request) error

func (fn AppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := fn(w, r); err != nil {
		var appErr *AppError
		if errors.As(err, &appErr) {
			slog.Error("request error",
				slog.String("path", r.URL.Path),
				slog.String("message", appErr.Message),
				slog.Any("err", appErr.Err),
			)
			code := appErr.Code
			if code == 0 {
				code = http.StatusInternalServerError
			}
			http.Error(w, appErr.Message, code)
			return
		}

		slog.Error("unhandled server error", slog.String("path", r.URL.Path), slog.Any("err", err))
		http.Error(w, "Internal server error: "+err.Error(), http.StatusInternalServerError)
	}
}

func redirectSuccess(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, path, http.StatusSeeOther)
}

func newBasePage(pool *pgx.ConnPool) (BasePage, error) {
	tables, err := getTables(pool)
	if err != nil {
		return BasePage{}, err
	}
	return BasePage{
		Tables: tables,
	}, nil
}

func getAllTableColumns(pool *pgx.ConnPool, tables []string) map[string][]string {
	res := make(map[string][]string, len(tables))
	for _, t := range tables {
		cols, err := getColumns(pool, t)
		if err == nil {
			names := make([]string, len(cols))
			for i, c := range cols {
				names[i] = c.Name
			}
			res[t] = names
		}
	}
	return res
}

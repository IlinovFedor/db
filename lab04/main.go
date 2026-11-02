package main

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx"
)

const (
	defaultPort = "8080"
	dbHost      = "localhost"
	dbPort      = 5432
	dbName      = "lab04_ilinov"
	dbUser      = "postgres"
	dbPassword  = "password"
)

func main() {
	httpAddress := ":" + defaultPort

	pool, err := pgx.NewConnPool(pgx.ConnPoolConfig{
		Host:           dbHost,
		Port:           dbPort,
		Database:       dbName,
		User:           dbUser,
		Password:       dbPassword,
		MaxConnections: 10,
	})
	if err != nil {
		slog.Error("cannot connect to postgres", slog.Any("err", err))
		os.Exit(1)
	}
	defer pool.Close()

	sqlMgr, err := NewSQLManager()
	if err != nil {
		slog.Error("cannot initialize sql manager", slog.Any("err", err))
		os.Exit(1)
	}

	if err := initDatabaseTables(pool, sqlMgr); err != nil {
		slog.Error("cannot initialize database tables", slog.Any("err", err))
		os.Exit(1)
	}

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"seq": func(n int) []int {
			res := make([]int, n)
			for i := range res {
				res[i] = i
			}
			return res
		},
		"eqStr": func(a, b string) bool { return a == b },
		"getRowVal": func(rows []map[string]string, rIdx int, colName string) string {
			if rIdx >= 0 && rIdx < len(rows) && rows[rIdx] != nil {
				return rows[rIdx][colName]
			}
			return ""
		},
	}

	views, err := NewViewRenderer(funcMap)
	if err != nil {
		slog.Error("cannot parse templates", slog.Any("err", err))
		os.Exit(1)
	}

	cssFS := http.FileServer(http.Dir("../css"))

	mux := http.NewServeMux()
	mux.Handle("GET /css/", http.StripPrefix("/css/", cssFS))

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/tables", http.StatusSeeOther)
	})

	mux.Handle("GET /tables", HandleTables(views, pool, sqlMgr))
	mux.Handle("GET /tables/{name}", HandleTables(views, pool, sqlMgr))

	mux.HandleFunc("GET /ddl", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ddl/create", http.StatusSeeOther)
	})
	mux.Handle("GET /ddl/{op}", HandleDDL(views, pool, sqlMgr))
	mux.Handle("POST /ddl/create", CreateTableHandler(views, pool, sqlMgr))
	mux.Handle("POST /ddl/alter", AlterTableHandler(views, pool, sqlMgr))
	mux.Handle("POST /ddl/truncate", TruncateTableHandler(views, pool, sqlMgr))
	mux.Handle("POST /ddl/drop", DropTableHandler(views, pool, sqlMgr))

	mux.HandleFunc("GET /dml", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dml/insert", http.StatusSeeOther)
	})
	mux.Handle("GET /dml/{op}", HandleDML(views, pool, sqlMgr))
	mux.Handle("POST /dml/insert", InsertHandler(views, pool, sqlMgr))
	mux.Handle("POST /dml/update", UpdateHandler(views, pool, sqlMgr))
	mux.Handle("POST /dml/delete", DeleteHandler(views, pool, sqlMgr))

	mux.Handle("GET /select", HandleSelect(views, pool, sqlMgr))
	mux.Handle("GET /select/{table}", HandleSelect(views, pool, sqlMgr))
	mux.Handle("POST /query/select", SelectHandler(views, pool, sqlMgr))

	mux.Handle("GET /join", HandleJoin(views, pool, sqlMgr))
	mux.Handle("POST /query/join", JoinHandler(views, pool, sqlMgr))

	mux.Handle("POST /reset", ResetHandler(views, pool, sqlMgr))

	slog.Info("starting server", slog.String("address", fmt.Sprintf("http://localhost:%s", defaultPort)))

	if err = http.ListenAndServe(httpAddress, mux); err != nil {
		slog.Error("cannot listen server", slog.Any("err", err))
		os.Exit(1)
	}
}

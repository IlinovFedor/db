package main

import (
	"html/template"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx"
)

const httpAddress = "localhost:8080"

type Response struct {
	Type string
	Rows any
}

func Root(htmlTemplate *template.Template) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		err := htmlTemplate.Execute(writer, nil)
		if err != nil {
			slog.Error("cannot execute template", slog.Any("err", err))
			writer.WriteHeader(
				http.StatusInternalServerError)
		}
	}
}

func Queries(htmlTemplate *template.Template, pool *pgx.ConnPool) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		opId := request.URL.Query().Get("opId")
		resp := new(Response)
		var err error
		switch opId {
		case "1":
			resp, err = execQuery(
				pool,
				"SELECT last_name, first_name, middle_name, phone_number, salary FROM lab2_employee_ilinov ORDER BY last_name, middle_name, first_name;")
		case "2":
			resp, err = execQuery(
				pool,
				"SELECT last_name, first_name, middle_name, address FROM lab2_employee_ilinov ORDER BY address, last_name, middle_name, first_name;")
		case "3":
			resp, err = execQuery(
				pool,
				"SELECT last_name, first_name, middle_name, date_start FROM lab2_employee_ilinov WHERE extract(days from now() - date_start) / 365 > 4 ORDER BY last_name, middle_name, first_name;")
		default:
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if err != nil {
			slog.Error("cannot get query", slog.Any("error", err), slog.Any("queryId", opId))
			writer.WriteHeader(
				http.StatusInternalServerError)
			return
		}
		resp.Type = opId

		if err := htmlTemplate.Execute(writer, resp); err != nil {
			slog.Error("cannot execute template", slog.Any("err", err))
			writer.WriteHeader(
				http.StatusInternalServerError)
			return
		}
	}
}

func execQuery(pool *pgx.ConnPool, query string) (*Response, error) {
	rows, err := pool.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]any, 0)
	for rows.Next() {
		row, err := rows.Values()
		if err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	resp := new(Response)
	resp.Rows = result
	return resp, nil
}

func main() {
	cssFS := http.FileServer(http.Dir("../css"))
	htmlTemplate, err := template.ParseFiles("index.html")
	if err != nil {
		slog.Error("cannot parse index.html", slog.Any("err", err))
		os.Exit(1)
	}

	pool, err := pgx.NewConnPool(pgx.ConnPoolConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "postgres",
		User:     "postgres",
		Password: "password",
	})

	if err != nil {
		slog.Error("cannot connect to postgres", slog.Any("err", err))
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /{$}", Root(htmlTemplate))
	mux.Handle("GET /css/", http.StripPrefix("/css/", cssFS))
	mux.Handle("GET /query", Queries(htmlTemplate, pool))

	slog.Info("starting server", slog.Any("address", httpAddress))

	if err = http.ListenAndServe(httpAddress, mux); err != nil {
		slog.Error("cannot listen server", slog.Any("err", err), slog.Any("address", httpAddress))
		os.Exit(1)
	}
}

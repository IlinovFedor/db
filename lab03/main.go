package main

import (
	"html/template"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx"
)

type Response struct {
	Type string
	Rows any
}

func Root(htmlTemplate *template.Template) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodGet:
			err := htmlTemplate.Execute(writer, nil)
			if err != nil {
				slog.Error("cannot execute template", err)
				writer.WriteHeader(http.StatusInternalServerError)
			}
		default:
			writer.WriteHeader(http.StatusNotImplemented)
		}
	}
}

func Queries(htmlTemplate *template.Template, pool *pgx.ConnPool) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodGet:
			opId := request.URL.Query().Get("opId")
			resp := new(Response)
			var err error
			switch opId {
			case "1":
				resp, err = query1(
					pool,
					"SELECT last_name, first_name, middle_name, phone_number, salary FROM lab2_employee_ilinov ORDER BY last_name, middle_name, first_name;")
			case "2":
				resp, err = query1(
					pool,
					"SELECT last_name, first_name, middle_name, address FROM lab2_employee_ilinov ORDER BY address, last_name, middle_name, first_name;")
			case "3":
				resp, err = query1(
					pool,
					"SELECT last_name, first_name, middle_name, date_start FROM lab2_employee_ilinov WHERE extract(days from now() - date_start) / 365 > 4 ORDER BY last_name, middle_name, first_name;")
			}
			resp.Type = opId
			if err != nil {
				slog.Error("cannot get query", slog.Any("error", err), slog.Any("queryId", opId))
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}

			if err := htmlTemplate.Execute(writer, resp); err != nil {
				slog.Error("cannot execute template", err)
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}
		default:
			writer.WriteHeader(http.StatusNotImplemented)
		}
	}
}

func query1(pool *pgx.ConnPool, query string) (*Response, error) {
	exec, err := pool.Query(query)
	if err != nil {
		return nil, err
	}

	rows := make([]any, 0)
	for exec.Next() {
		values, err := exec.Values()
		if err != nil {
			return nil, err
		}
		rows = append(rows, values)
	}
	exec.Close()

	resp := new(Response)
	resp.Rows = rows
	return resp, nil
}

func main() {
	cssFS := http.FileServer(http.Dir("../css"))
	htmlTemplate, err := template.ParseFiles("index.html")
	if err != nil {
		slog.Error("cannot parse index.html", err)
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
		slog.Error("cannot connect to postgres", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.Handle("/css/", http.StripPrefix("/css/", cssFS))
	mux.Handle("/", Root(htmlTemplate))
	mux.Handle("/query", Queries(htmlTemplate, pool))

	if http.ListenAndServe("localhost:8080", mux) != nil {
		slog.Error("cannot listen localhost:8080", err)
		os.Exit(1)
	}
}

package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	dashboardProvider "github.com/jeefy/metrics-sidecar/pkg/api/dashboard"
	_ "github.com/mattn/go-sqlite3"
)

func ApiManager(r *mux.Router, db *sql.DB) {
	r.HandleFunc("/", RootHandler)
	dashboardRouter := r.PathPrefix("/api/v1/dashboard").Subrouter()
	dashboardProvider.DashboardRouter(dashboardRouter, db)

	r.PathPrefix("/").HandlerFunc(DefaultHandler)

}

func RootHandler(w http.ResponseWriter, r *http.Request) {
	msg := fmt.Sprintf("URL: %s", r.URL)
	log.Println(msg)
	w.Write([]byte(msg))
}

func DefaultHandler(w http.ResponseWriter, r *http.Request) {
	msg := fmt.Sprintf("URL: %s", r.URL)
	log.Println(msg)
	w.Write([]byte(msg))
}

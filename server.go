package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	sideapi "github.com/jeefy/metrics-sidecar/pkg/api"
	sidedb "github.com/jeefy/metrics-sidecar/pkg/database"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"

	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	var kubeconfig *string
	var dbFile *string
	var refreshInterval *int
	var maxWindow *int

	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	dbFile = flag.String("db-file", ":memory:", "What file to use as a SQLite3 database. Defaults to ':memory:'")
	refreshInterval = flag.Int("refresh-interval", 10, "Frequency (in seconds) to update the metrics database. Defaults to '5'")
	maxWindow = flag.Int("max-window", 15, "Window of time you wish to retain records (in minutes). Defaults to '15'")

	flag.Parse()

	// This should only be run in-cluster so...
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Kubernetes host: ", config.Host)

	// Generate the metrics client
	clientset, err := metricsclient.NewForConfig(config)
	if err != nil {
		fmt.Println(err.Error())
	}

	// Create the db "connection"
	db, err := sql.Open("sqlite3", *dbFile)
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	// Populate tables
	err = sidedb.CreateDatabase(db)
	if err != nil {
		panic(err.Error())
	}

	go func() {
		r := mux.NewRouter()
		sideapi.ApiManager(r, db)
		// Bind to a port and pass our router in
		http.ListenAndServe(":8000", r)
	}()

	// Start the machine. Scrape every refreshInterval
	ticker := time.NewTicker(time.Duration(*refreshInterval) * time.Second)
	quit := make(chan struct{})

	for {
		select {
		case <-quit:
			ticker.Stop()
			return

		case t := <-ticker.C:
			err = nil
			nodeMetrics, err := clientset.Metrics().NodeMetricses().List(v1.ListOptions{})
			if err != nil {
				fmt.Println(err.Error())
				break
			}

			podMetrics, err := clientset.Metrics().PodMetricses("").List(v1.ListOptions{})
			if err != nil {
				fmt.Println(err.Error())
				break
			}

			// Insert scrapes into DB
			err = sidedb.UpdateDatabase(db, nodeMetrics, podMetrics)
			if err != nil {
				fmt.Println("error updating database")
				fmt.Println(err.Error())
				break
			}

			// Delete rows outside of the maxWindow time
			err = sidedb.CullDatabase(db, maxWindow)
			if err != nil {
				fmt.Println("error culling database")
				fmt.Println(err.Error())
				break
			}

			fmt.Println(fmt.Sprintf("%v - db updated", t))
		}
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

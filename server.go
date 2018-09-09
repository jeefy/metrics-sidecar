package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	sideDb "github.com/jeefy/metrics-sidecar/pkg/database"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
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
	refreshInterval = flag.Int("refresh-interval", 5, "Frequency (in seconds) to update the metrics database. Defaults to '5'")
	maxWindow = flag.Int("max-window", 15, "Window of time you wish to retain records (in minutes). Defaults to '15'")

	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Kubernetes host: ", config.Host)

	// Generate the metrics client
	clientset, err := metricsclient.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Create the db "connection"
	db, err := sql.Open("sqlite3", *dbFile)
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	// Populate tables
	sideDb.CreateDatabase(db)

	// Start the machine. Scrape every refreshInterval
	ticker := time.NewTicker(time.Duration(*refreshInterval) * time.Second)
	quit := make(chan struct{})

	for {
		select {
		case <-ticker.C:
			t := <-ticker.C
			nodeMetrics, err := clientset.Metrics().NodeMetricses().List(v1.ListOptions{})
			if err != nil {
				panic(err.Error())
			}

			podMetrics, err := clientset.Metrics().PodMetricses("").List(v1.ListOptions{})
			if err != nil {
				panic(err.Error())
			}

			// Insert scrapes into DB
			sideDb.UpdateDatabase(db, nodeMetrics, podMetrics)

			// Delete rows outside of the maxWindow time
			sideDb.CullDatabase(db, maxWindow)

			fmt.Println(t, " - db updated")
		case <-quit:
			ticker.Stop()
			return
		}
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

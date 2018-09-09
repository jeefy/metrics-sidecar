package database

import (
	"database/sql"
	"fmt"

	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

/*
	CreateDatabase: Creates tables for node and pod metrics
*/
func CreateDatabase(db *sql.DB) {
	sqlStmt := `
	create table if not exists nodes (name text, cpu text, memory text, storage text, time datetime);
	create table if not exists pods (name text, container text, cpu text, memory text, storage text, time datetime);
	`
	_, err := db.Exec(sqlStmt)
	if err != nil {
		panic(err.Error())
	}
}

/*
	UpdateDatabase: Takes nodeMetrics and podMetrics and inserts the data
*/
func UpdateDatabase(db *sql.DB, nodeMetrics *v1beta1.NodeMetricsList, podMetrics *v1beta1.PodMetricsList) {
	tx, err := db.Begin()
	if err != nil {
		panic(err.Error())
	}
	stmt, err := tx.Prepare("insert into nodes(name, cpu, memory, storage, time) values(?, ?, ?, ?, datetime('now'))")
	if err != nil {
		panic(err.Error())
	}
	defer stmt.Close()

	for _, v := range nodeMetrics.Items {
		_, err = stmt.Exec(v.Name, v.Usage.Cpu().String(), v.Usage.Memory().String(), v.Usage.StorageEphemeral().String())
		if err != nil {
			panic(err.Error())
		}
	}

	stmt, err = tx.Prepare("insert into pods(name, container, cpu, memory, storage, time) values(?, ?, ?, ?, ?, datetime('now'))")
	if err != nil {
		panic(err.Error())
	}
	defer stmt.Close()

	for _, v := range podMetrics.Items {
		for _, u := range v.Containers {
			_, err = stmt.Exec(v.Name, u.Name, u.Usage.Cpu().String(), u.Usage.Memory().String(), u.Usage.StorageEphemeral().String())
			if err != nil {
				panic(err.Error())
			}
		}
	}

	tx.Commit()
}

/*
	CullDatabase: Deletes rows from nodes and pods based on a time window.
*/
func CullDatabase(db *sql.DB, window *int) {
	var sqlStmt string
	sqlStmt = fmt.Sprintf("delete from nodes where time <= datetime('now','-%d minutes');", *window)
	_, err := db.Exec(sqlStmt)
	if err != nil {
		panic(err.Error())
	}

	sqlStmt = fmt.Sprintf("delete from pods where time <= datetime('now','-%d minutes');", *window)
	_, err = db.Exec(sqlStmt)
	if err != nil {
		panic(err.Error())
	}
}

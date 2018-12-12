package database

import (
	"database/sql"
	"fmt"

	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

/*
	CreateDatabase: Creates tables for node and pod metrics
*/
func CreateDatabase(db *sql.DB) error {
	sqlStmt := `
	create table if not exists nodes (uid text, name text, cpu text, memory text, storage text, time datetime);
	create table if not exists pods (uid text, name text, namespace text, container text, cpu text, memory text, storage text, time datetime);
	`
	_, err := db.Exec(sqlStmt)
	if err != nil {
		return err
	}

	return nil
}

/*
	UpdateDatabase: Takes nodeMetrics and podMetrics and inserts the data
*/
func UpdateDatabase(db *sql.DB, nodeMetrics *v1beta1.NodeMetricsList, podMetrics *v1beta1.PodMetricsList) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("insert into nodes(uid, name, cpu, memory, storage, time) values(?, ?, ?, ?, ?, datetime('now'))")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, v := range nodeMetrics.Items {
		_, err = stmt.Exec(v.UID, v.Name, v.Usage.Cpu().String(), v.Usage.Memory().String(), v.Usage.StorageEphemeral().String())
		if err != nil {
			return err
		}
	}

	stmt, err = tx.Prepare("insert into pods(uid, name, namespace, container, cpu, memory, storage, time) values(?, ?, ?, ?, ?, ?, ?, datetime('now'))")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, v := range podMetrics.Items {
		for _, u := range v.Containers {
			_, err = stmt.Exec(v.UID, v.Name, v.Namespace, u.Name, u.Usage.Cpu().String(), u.Usage.Memory().String(), u.Usage.StorageEphemeral().String())
			if err != nil {
				return err
			}
		}
	}

	tx.Commit()
	return nil
}

/*
	CullDatabase: Deletes rows from nodes and pods based on a time window.
*/
func CullDatabase(db *sql.DB, window *int) error {
	var sqlStmt string

	sqlStmt = fmt.Sprintf("delete from nodes where time <= datetime('now','-%d minutes');", *window)
	_, err := db.Exec(sqlStmt)
	if err != nil {
		return err
	}

	sqlStmt = fmt.Sprintf("delete from pods where time <= datetime('now','-%d minutes');", *window)
	_, err = db.Exec(sqlStmt)
	if err != nil {
		return err
	}

	return nil
}

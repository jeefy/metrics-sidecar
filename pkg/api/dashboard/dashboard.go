package provider

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	metricsApi "github.com/kubernetes/dashboard/src/app/backend/integration/metric/api"
)

func DashboardRouter(r *mux.Router, db *sql.DB) {
	r.Path("/nodes/{Name}/metrics/{MetricName}/{Whatever}").HandlerFunc(nodeHandler(db))
	r.Path("/namespaces/{Namespace}/pod-list/{Name}/metrics/{MetricName}/{Whatever}").HandlerFunc(podHandler(db))
	r.PathPrefix("/").HandlerFunc(defaultHandler)
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	msg := fmt.Sprintf("%v - URL: %s", time.Now(), r.URL)
	fmt.Println(msg)
	w.Write([]byte(msg))
}

func nodeHandler(db *sql.DB) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		msg := fmt.Sprintf("%v - URL: %s", time.Now(), r.URL)
		fmt.Println(msg)
		vars := mux.Vars(r)

		resp, err := getNodeMetrics(db, vars["MetricName"], metricsApi.ResourceSelector{
			Namespace:    "",
			ResourceName: vars["Name"],
		})

		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf("Node Metrics Error - %v", err.Error())))
		}

		j, err := json.Marshal(resp)

		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf("JSON Error - %v", err.Error())))
		}

		w.Write([]byte(j))
	}

	return http.HandlerFunc(fn)
}

func podHandler(db *sql.DB) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		msg := fmt.Sprintf("%v - URL: %s", time.Now(), r.URL)
		fmt.Println(msg)
		vars := mux.Vars(r)

		resp, err := getPodMetrics(db, vars["MetricName"], metricsApi.ResourceSelector{
			Namespace:    vars["Namespace"],
			ResourceName: vars["Name"],
		})

		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf("Node Metrics Error - %v", err.Error())))
		}

		j, err := json.Marshal(resp)

		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf("JSON Error - %v", err.Error())))
		}

		w.Write([]byte(j))
	}

	return http.HandlerFunc(fn)
}

/*
	getPodMetrics: With a database connection and a resource selector
	Queries SQLite and returns a list of metrics.
*/
func getPodMetrics(db *sql.DB, metricName string, selector metricsApi.ResourceSelector) (metricsApi.SidecarMetricResultList, error) {
	query := ""
	multiplier := uint64(1000)
	if metricName == "cpu" {
		query = "select sum(cpu), name, uid, time from pods where "
		multiplier = uint64(1)
	} else {
		//default to metricName == "memory/usage"
		metricName = "memory"
		query = "select sum(memory), name, uid, time from pods where "
	}

	if selector.Namespace != "" {
		query = fmt.Sprintf(query+" namespace='%v'", selector.Namespace)
	} else {
		query = query + " namespace='default'"
	}

	if selector.ResourceName != "" {
		if strings.ContainsAny(selector.ResourceName, ",") {
			query = fmt.Sprintf(query+" and name in (%v)", "'"+strings.Join(strings.Split(selector.ResourceName, ","), "', '")+"'")
		} else {
			query = fmt.Sprintf(query+" and name='%v'", selector.ResourceName)
		}
	}
	if selector.UID != "" {
		query = fmt.Sprintf(query+" uid='%v'", selector.UID)
	}

	query = query + " group by name, time order by namespace, name, time;"

	resultList := make(map[string]metricsApi.SidecarMetric)

	rows, err := db.Query(query)
	if err != nil {
		return metricsApi.SidecarMetricResultList{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var metricValue string
		var pod string
		var metricTime string
		var uid string
		err = rows.Scan(&metricValue, &pod, &uid, &metricTime)
		if err != nil {
			return metricsApi.SidecarMetricResultList{}, err
		}

		layout := "2006-01-02T15:04:05Z"
		t, err := time.Parse(layout, metricTime)
		if err != nil {
			return metricsApi.SidecarMetricResultList{}, err
		}

		v, err := strconv.ParseUint(metricValue, 10, 64)

		newMetric := metricsApi.MetricPoint{
			Timestamp: t,
			Value:     v * multiplier,
		}

		if _, ok := resultList[pod]; ok {
			metricThing := resultList[pod]
			metricThing.AddMetricPoint(newMetric)
			resultList[pod] = metricThing
		} else {
			resultList[pod] = metricsApi.SidecarMetric{
				MetricName:   metricName,
				MetricPoints: []metricsApi.MetricPoint{newMetric},
				DataPoints:   []metricsApi.DataPoint{},
				UIDs: []string{
					pod,
				},
			}
		}
	}
	err = rows.Err()
	if err != nil {
		return metricsApi.SidecarMetricResultList{}, err
	}

	result := metricsApi.SidecarMetricResultList{}
	for _, v := range resultList {
		result.Items = append(result.Items, v)
	}

	return result, nil
}

/*
	getNodeMetrics: With a database connection and a resource selector
	Queries SQLite and returns a list of metrics.
*/
func getNodeMetrics(db *sql.DB, metricName string, selector metricsApi.ResourceSelector) (metricsApi.SidecarMetricResultList, error) {
	query := ""
	multiplier := uint64(1000)
	stripNum := 2
	if metricName == "cpu" {
		query = "select cpu, name, uid, time from nodes "
		multiplier = uint64(1)
		stripNum = 1
	} else {
		metricName = "memory"
		//default to metricName == "memory/usage"
		query = "select memory, name, uid, time from nodes "
	}

	if selector.ResourceName != "" {
		if strings.ContainsAny(selector.ResourceName, ",") {
			query = fmt.Sprintf(query+" where name in (?)", "'"+strings.Join(strings.Split(selector.ResourceName, ","), "', '")+"'")
		} else {
			query = fmt.Sprintf(query+" where name='%v'", selector.ResourceName)
		}
	}

	query = query + " group by name, time order by name, time;"

	resultList := make(map[string]metricsApi.SidecarMetric)

	rows, err := db.Query(query)
	if err != nil {
		return metricsApi.SidecarMetricResultList{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var metricValue string
		var node string
		var metricTime string
		var uid string
		err = rows.Scan(&metricValue, &node, &uid, &metricTime)
		if err != nil {
			return metricsApi.SidecarMetricResultList{}, err
		}

		layout := "2006-01-02T15:04:05Z"
		t, err := time.Parse(layout, metricTime)
		if err != nil {
			return metricsApi.SidecarMetricResultList{}, err
		}

		v, err := strconv.ParseUint(metricValue[0:len(metricValue)-stripNum], 10, 64)

		newMetric := metricsApi.MetricPoint{
			Timestamp: t,
			Value:     v * multiplier,
		}

		if _, ok := resultList[node]; ok {
			metricThing := resultList[node]
			metricThing.AddMetricPoint(newMetric)
			resultList[node] = metricThing
		} else {
			resultList[node] = metricsApi.SidecarMetric{
				MetricName:   metricName,
				MetricPoints: []metricsApi.MetricPoint{newMetric},
				DataPoints:   []metricsApi.DataPoint{},
				UIDs: []string{
					node,
				},
			}
		}
	}
	err = rows.Err()
	if err != nil {
		return metricsApi.SidecarMetricResultList{}, err
	}

	result := metricsApi.SidecarMetricResultList{}
	for _, v := range resultList {
		result.Items = append(result.Items, v)
	}

	return result, nil
}

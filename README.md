# Metrics-Sidecar
Small binary to scrape and store a small window of metrics from the Metrics Server in Kubernetes.

## Command-Line Arguments
| Flag  | Description  | Default  |
|---|---|---|
| kubeconfig  | Absolute path to the kubeconfig file  | `~/.kube`  |
| db-file  | What file to use as a SQLite3 database.  |  `:memory:` |
| refresh-interval | Frequency (in seconds) to update the metrics database.  | `5` |
| max-window | Window of time you wish to retain records (in minutes). | `15` |

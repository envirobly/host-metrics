# host-metrics

Custom Prometheus exporter for Envirobly instances.

## Development

```sh
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promhttp
go get github.com/shirou/gopsutil/cpu
go get github.com/shirou/gopsutil/mem

# Running
go run main.go
```

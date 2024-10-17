package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

// Metrics structure
type Metrics struct {
	ramUsage prometheus.Gauge
	cpuUsage prometheus.Gauge
}

// NewMetrics initializes Prometheus metrics
func NewMetrics() *Metrics {
	return &Metrics{
		ramUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "envirobly:node:mem_utilization:percent",
			Help: "Total RAM utilization in percent",
		}),
		cpuUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "envirobly:node:cpu_utilization:percent",
			Help: "Total CPU utilization in percent (across all cores)",
		}),
	}
}

// RegisterMetrics registers metrics with Prometheus custom registry
func (m *Metrics) RegisterMetrics(reg *prometheus.Registry) {
	reg.MustRegister(m.ramUsage)
	reg.MustRegister(m.cpuUsage)
}

// roundToTwoDecimals rounds a float64 to two decimal places
func roundToTwoDecimals(value float64) float64 {
	return math.Round(value*100) / 100
}

// CollectMetrics collects RAM and CPU metrics
func (m *Metrics) CollectMetrics() {
	for {
		// Collect RAM usage
		vmStat, err := mem.VirtualMemory()
		if err != nil {
			log.Printf("Error collecting RAM metrics: %v", err)
		} else {
			m.ramUsage.Set(roundToTwoDecimals(vmStat.UsedPercent))
		}

		// Collect CPU usage
		cpuPercents, err := cpu.Percent(0, false)
		if err != nil {
			log.Printf("Error collecting CPU metrics: %v", err)
		} else {
			// Get total CPU usage across all cores
			if len(cpuPercents) > 0 {
				m.cpuUsage.Set(roundToTwoDecimals(cpuPercents[0]))
			}
		}

		time.Sleep(5 * time.Second) // Collect every 5 seconds
	}
}

func main() {
	// Create a custom registry to avoid the default Go metrics
	registry := prometheus.NewRegistry()

	metrics := NewMetrics()
	metrics.RegisterMetrics(registry)

	// Start metric collection in a goroutine
	go metrics.CollectMetrics()

	// Expose only the custom metrics via HTTP endpoint
	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	fmt.Println("Starting server on :2112")
	log.Fatal(http.ListenAndServe(":2112", nil))
}

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
			Name: "ram_usage_percent",
			Help: "Total RAM utilization in percent",
		}),
		cpuUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "cpu_usage_percent",
			Help: "Total CPU utilization in percent (across all cores)",
		}),
	}
}

// RegisterMetrics registers metrics with Prometheus
func (m *Metrics) RegisterMetrics() {
	prometheus.MustRegister(m.ramUsage)
	prometheus.MustRegister(m.cpuUsage)
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
	metrics := NewMetrics()
	metrics.RegisterMetrics()

	// Start metric collection in a goroutine
	go metrics.CollectMetrics()

	// Expose metrics via HTTP endpoint for Prometheus to scrape
	http.Handle("/metrics", promhttp.Handler())
	fmt.Println("Starting server on :2112")
	log.Fatal(http.ListenAndServe(":2112", nil))
}

package main

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
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
	zfsUsage *prometheus.GaugeVec // For ZFS pool utilization
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
		zfsUsage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "zfs_pool_usage_percent",
				Help: "ZFS pool utilization in percent (capacity)",
			},
			[]string{"pool"},
		),
	}
}

// RegisterMetrics registers metrics with Prometheus
func (m *Metrics) RegisterMetrics(reg *prometheus.Registry) {
	reg.MustRegister(m.ramUsage)
	reg.MustRegister(m.cpuUsage)
	reg.MustRegister(m.zfsUsage)
}

// roundToTwoDecimals rounds a float64 to two decimal places
func roundToTwoDecimals(value float64) float64 {
	return math.Round(value*100) / 100
}

// CollectZFSMetrics collects ZFS pool utilization metrics using the 'cap' percentage from 'zpool list'
func (m *Metrics) CollectZFSMetrics() {
	for {
		// Execute 'zpool list' to get ZFS pool utilization percentages (capacity)
		out, err := exec.Command("zpool", "list", "-H", "-o", "name,cap").Output()
		if err != nil {
			log.Printf("Error collecting ZFS metrics: %v", err)
			continue
		}

		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) != 2 {
				continue // Skip invalid lines
			}

			poolName := fields[0]
			capPercentStr := strings.TrimSuffix(fields[1], "%") // Remove '%' symbol

			capPercent, err := strconv.ParseFloat(capPercentStr, 64)
			if err != nil {
				log.Printf("Error parsing capacity percentage for pool %s: %v", poolName, err)
				continue
			}

			// Set the metric value for this pool
			m.zfsUsage.WithLabelValues(poolName).Set(roundToTwoDecimals(capPercent))
		}

		if err := scanner.Err(); err != nil {
			log.Printf("Error scanning ZFS output: %v", err)
		}

		time.Sleep(10 * time.Second) // Collect every 10 seconds
	}
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

	// Start metric collection in separate goroutines
	go metrics.CollectMetrics()
	go metrics.CollectZFSMetrics()

	// Expose only the custom metrics via HTTP endpoint
	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	fmt.Println("Starting server on :2112")
	log.Fatal(http.ListenAndServe(":2112", nil))
}

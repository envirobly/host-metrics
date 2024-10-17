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
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
)

// Metrics structure
type Metrics struct {
	ramUsage prometheus.Gauge
	cpuUsage prometheus.Gauge
	swapUsage prometheus.Gauge
	zfsUsage *prometheus.GaugeVec
	fsUsage  *prometheus.GaugeVec
	netBytesSent *prometheus.GaugeVec
	netBytesRecv *prometheus.GaugeVec
}

// NewMetrics initializes Prometheus metrics
func NewMetrics() *Metrics {
	return &Metrics{
		ramUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "envirobly_ram_usage_percent",
			Help: "Total RAM utilization in percent",
		}),
		cpuUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "envirobly_cpu_usage_percent",
			Help: "Total CPU utilization in percent (across all cores)",
		}),
		swapUsage: prometheus.NewGauge(prometheus.GaugeOpts{
	    Name: "envirobly_swap_usage_percent",
	    Help: "Total swap memory utilization in percent",
		}),
		zfsUsage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "envirobly_zpool_usage_percent",
				Help: "ZFS pool utilization in percent (capacity)",
			},
			[]string{"pool"},
		),
		fsUsage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "envirobly_filesystem_usage_percent",
				Help: "Filesystem utilization in percent",
			},
			[]string{"filesystem", "mountpoint"},
		),
		netBytesSent: prometheus.NewGaugeVec(
	    prometheus.GaugeOpts{
	      Name: "envirobly_network_bytes_sent_total",
	      Help: "Total bytes transmitted on network interfaces",
	    },
	    []string{"interface"},
		),
		netBytesRecv: prometheus.NewGaugeVec(
	    prometheus.GaugeOpts{
	      Name: "envirobly_network_bytes_recv_total",
	      Help: "Total bytes received on network interfaces",
	    },
	    []string{"interface"},
		),
	}
}

// RegisterMetrics registers metrics with Prometheus
func (m *Metrics) RegisterMetrics(reg *prometheus.Registry) {
	reg.MustRegister(m.ramUsage)
	reg.MustRegister(m.cpuUsage)
	reg.MustRegister(m.swapUsage)
	reg.MustRegister(m.zfsUsage)
	reg.MustRegister(m.fsUsage)
	reg.MustRegister(m.netBytesSent)
	reg.MustRegister(m.netBytesRecv)
}

// roundToTwoDecimals rounds a float64 to two decimal places
func roundToTwoDecimals(value float64) float64 {
	return math.Round(value*100) / 100
}

func isExcludedMountPoint(mountpoint string) bool {
	return strings.HasPrefix(mountpoint, "/boot/efi") ||
		strings.HasPrefix(mountpoint, "/var/envirobly/zpools") ||
		strings.HasPrefix(mountpoint, "/var/lib/docker/volumes")
}

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

		// Get swap memory statistics
		swapStat, err := mem.SwapMemory()
		if err != nil {
			log.Printf("Error collecting swap memory metrics: %v", err)
		} else {
			// Set the swap usage metric
			m.swapUsage.Set(roundToTwoDecimals(swapStat.UsedPercent))
		}

		// Get network I/O stats
		netIOStats, err := net.IOCounters(true)
		if err != nil {
			log.Printf("Error collecting network I/O stats: %v", err)
		} else {
			// Loop through the stats and filter for interfaces that start with "ens"
			for _, stat := range netIOStats {
				if strings.HasPrefix(stat.Name, "ens") {
					// Set the metrics for transmitted and received bytes
					m.netBytesSent.WithLabelValues(stat.Name).Set(float64(stat.BytesSent))
					m.netBytesRecv.WithLabelValues(stat.Name).Set(float64(stat.BytesRecv))
				}
			}
		}

		// Get filesystem utilization metrics
		partitions, err := disk.Partitions(false) // Get all mounted partitions
		if err != nil {
			log.Printf("Error collecting filesystem partitions: %v", err)
		} else {
			for _, partition := range partitions {
				if isExcludedMountPoint(partition.Mountpoint) {
					continue
				}

				usageStat, err := disk.Usage(partition.Mountpoint) // Get usage for each mount point
				if err != nil {
					log.Printf("Error collecting filesystem usage for %s: %v", partition.Mountpoint, err)
					continue
				}

				// Set the metric value for this filesystem
				m.fsUsage.
					WithLabelValues(partition.Device, partition.Mountpoint).
					Set(roundToTwoDecimals(usageStat.UsedPercent))
			}
		}

		// Get ZFS pool utilization metrics
		// Execute 'zpool list' to get ZFS pool utilization percentages (capacity)
		out, err := exec.Command("zpool", "list", "-H", "-o", "name,cap").Output()
		if err != nil {
			log.Printf("Error collecting ZFS metrics: %v", err)
		} else {
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
		}

		time.Sleep(7 * time.Second) // Collect every 7 seconds
	}
}

func main() {
	// Create a custom registry to avoid the default Go metrics
	registry := prometheus.NewRegistry()

	metrics := NewMetrics()
	metrics.RegisterMetrics(registry)

	// Start metric collection in separate goroutines
	go metrics.CollectMetrics()

	// Expose only the custom metrics via HTTP endpoint
	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	fmt.Println("Starting server on :63107")
	log.Fatal(http.ListenAndServe(":63107", nil))
}

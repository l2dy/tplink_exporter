package main

import (
	"fmt"
	"github.com/maesoser/tplink_exporter/macdb"
	"github.com/maesoser/tplink_exporter/tplink"
	"github.com/prometheus/client_golang/prometheus"
	"log"
	"sync"
)

//Define a struct for you collector that contains pointers
//to prometheus descriptors for each metric you wish to expose.
//Note you can also include fields of other types if they provide utility
//but we just won't be exposing them as metrics.
type routerCollector struct {
	txWANTraffic *prometheus.Desc
	rxWANTraffic *prometheus.Desc
	LANTraffic   *prometheus.Desc
	LANPackets   *prometheus.Desc
	LANLeases    *prometheus.Desc

	router  *tplink.Router
	macs    macdb.DB
	vendors macdb.DB

	mutex sync.Mutex
}

//You must create a constructor for you collector that
//initializes every descriptor and returns a pointer to the collector
func newRouterCollector(router *tplink.Router, macs, vendors macdb.DB) *routerCollector {

	c := routerCollector{}

	c.txWANTraffic = prometheus.NewDesc(
		"tplink_wan_tx_kbytes",
		"Total kbytes transmitted",
		nil, nil,
	)

	c.rxWANTraffic = prometheus.NewDesc(
		"tplink_wan_rx_kbytes",
		"Total kbytes received",
		nil, nil,
	)

	c.LANTraffic = prometheus.NewDesc(
		"tplink_lan_traffic_kbytes",
		"KBytes sent/received per device",
		[]string{"name", "addr", "mac"}, nil,
	)

	c.LANPackets = prometheus.NewDesc(
		"tplink_lan_traffic_packets",
		"Packets sent/received per device",
		[]string{"name", "addr", "mac"}, nil,
	)

	c.LANLeases = prometheus.NewDesc(
		"tplink_lan_lease_seconds",
		"Lease seconds left",
		[]string{"name", "addr", "mac"}, nil,
	)

	c.macs = macs
	c.vendors = vendors
	c.router = router

	return &c

}

//Each and every collector must implement the Describe function.
//It essentially writes all descriptors to the prometheus desc channel.
func (collector *routerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.txWANTraffic
	ch <- collector.rxWANTraffic
	ch <- collector.LANTraffic
	ch <- collector.LANPackets
	ch <- collector.LANLeases

}

func (collector *routerCollector) scrape(ch chan<- prometheus.Metric) error {
	err := collector.router.Login()
	if err != nil {
		return fmt.Errorf("Error logging: %v", err)
	}
	rx, tx, err := collector.router.GetWANTraffic()
	if err != nil {
		return fmt.Errorf("Error getting WAN metrics: %v", err)
	}
	ch <- prometheus.MustNewConstMetric(collector.rxWANTraffic, prometheus.CounterValue, rx)
	ch <- prometheus.MustNewConstMetric(collector.txWANTraffic, prometheus.CounterValue, tx)

	clients, err := collector.router.GetClients()
	if err != nil {
		return fmt.Errorf("Error getting WAN metrics: %v", err)
	}
	clients, err = collector.router.GetLANTraffic(clients)
	if err != nil {
		return fmt.Errorf("Error getting LAN metrics: %v", err)
	}
	for _, client := range clients {
		name := macdb.Lookup(client.MAC, collector.macs, collector.vendors)
		if len(name) == 0 {
			name = client.Name
		}
		ch <- prometheus.MustNewConstMetric(
			collector.LANTraffic,
			prometheus.GaugeValue,
			client.Bytes,
			name, client.Addr, client.MAC)
		ch <- prometheus.MustNewConstMetric(
			collector.LANLeases,
			prometheus.GaugeValue,
			client.Lease,
			name, client.Addr, client.MAC)
		ch <- prometheus.MustNewConstMetric(
			collector.LANPackets,
			prometheus.GaugeValue,
			client.Packets,
			name, client.Addr, client.MAC)

	}
	return nil
	//router.Logout()
}

//Collect implements required collect function for all promehteus collectors
func (collector *routerCollector) Collect(ch chan<- prometheus.Metric) {

	collector.mutex.Lock()
	defer collector.mutex.Unlock()

	err := collector.scrape(ch)
	if err != nil {
		log.Println("Error scraping data for router", err)
	}

	//ch <- prometheus.MustNewConstMetric(collector.barMetric, prometheus.CounterValue, metricValue)

}

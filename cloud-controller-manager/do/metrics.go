/*
Copyright 2023 DigitalOcean

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package do

import "github.com/prometheus/client_golang/prometheus"

type firewallOperation string

const (
	firewallOperationGetByID   = "get_by_id"
	firewallOperationGetByList = "get_by_list"
	firewallOperationCreate    = "create"
	firewallOperationUpdate    = "update"
)

type metrics struct {
	host                 string
	apiOperationDuration *prometheus.HistogramVec
	apiOperationsTotal   *prometheus.CounterVec
	resourceSyncDuration *prometheus.HistogramVec
	resourceSyncsTotal   *prometheus.CounterVec
	reconcileDuration    *prometheus.HistogramVec
	reconcilesTotal      *prometheus.CounterVec
}

const (
	// apiOperationDurationWidth of 4.5 represents 90(duration in sec)/20(bucket count -1)
	apiOperationDurationWidth float64 = 4.5
	// resourceSyncDurationWidth of 15 represents 300(duration in sec)/20(bucket count -1)
	resourceSyncDurationWidth float64 = 15
	// reconcileDurationWidth of 15 represents 300(duration in sec)/20(bucket count -1)
	reconcileDurationWidth float64 = 15
)

// create metrics
var (
	apiOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "firewall",
			Name:      "api_operation_duration_seconds",
			Help:      "Histogram for tracking the duration of firewall API operations.",
			Buckets:   prometheus.LinearBuckets(1, apiOperationDurationWidth, 21),
		},
		[]string{"operation", "result", "http_response_code"},
	)
	apiOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "firewall",
			Name:      "api_operations_total",
			Help:      "The total number of firewall API operations executed.",
		},
		[]string{"operation", "result", "http_response_code"},
	)
	resourceSyncDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "firewall",
			Name:      "resource_sync_duration_seconds",
			Help:      "Histogram for tracking the duration of remote firewall resource synchronizations.",
			Buckets:   prometheus.LinearBuckets(1, resourceSyncDurationWidth, 21),
		},
		[]string{"result"},
	)
	resourceSyncsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "firewall",
			Name:      "resource_syncs_total",
			Help:      "The total number of remote firewall resource synchronizations executed.",
		},
		[]string{"result"},
	)
	reconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "firewall",
			Name:      "reconcile_duration_seconds",
			Help:      "Histogram for tracking the duration of firewall reconciles.",
			Buckets:   prometheus.LinearBuckets(1, reconcileDurationWidth, 21),
		},
		[]string{"result", "error_type"},
	)
	reconcilesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "firewall",
			Name:      "reconciles_total",
			Help:      "The total number of firewall reconciles executed.",
		},
		[]string{"result", "error_type"},
	)
)

func newMetrics(host string) metrics {
	return metrics{
		host:                 host,
		apiOperationDuration: apiOperationDuration,
		apiOperationsTotal:   apiOperationsTotal,
		resourceSyncDuration: resourceSyncDuration,
		resourceSyncsTotal:   resourceSyncsTotal,
		reconcileDuration:    reconcileDuration,
		reconcilesTotal:      reconcilesTotal,
	}
}

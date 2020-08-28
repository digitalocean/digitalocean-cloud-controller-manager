/*
Copyright 2020 DigitalOcean

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

type metrics struct {
	host               string
	apiRequestDuration *prometheus.HistogramVec
	runLoopDuration    *prometheus.HistogramVec
	reconcileDuration  *prometheus.HistogramVec
}

const (
	// apiRequestDurationWidth of 4.5 represents 90(duration in sec)/20(bucket count -1)
	apiRequestDurationWidth float64 = 4.5
	// runLoopDurationWidth of 15 represents 300(duration in sec)/20(bucket count -1)
	runLoopDurationWidth float64 = 15
	// reconcileDurationWidth of 15 represents 300(duration in sec)/20(bucket count -1)
	reconcileDurationWidth float64 = 15
)

// create metrics
var (
	apiRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "firewall",
			Name:      "http_request_duration_seconds",
			Help:      "This is a histogram for tracking the duration of firewall API requests.",
			Buckets:   prometheus.LinearBuckets(1, apiRequestDurationWidth, 21),
		},
		[]string{"method", "code"},
	)
	runLoopDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "firewall",
			Name:      "run_loop_duration_seconds",
			Help:      "This is a histogram for tracking the duration of the run loop and whether it succeeds or not.",
			Buckets:   prometheus.LinearBuckets(1, runLoopDurationWidth, 21),
		},
		[]string{"success"},
	)
	reconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "firewall",
			Name:      "reconcile_duration_seconds",
			Help:      "The duration of time, in seconds, that it takes for the firewall reconcile to run and ensure that the firewall is reconciled.",
			Buckets:   prometheus.LinearBuckets(1, reconcileDurationWidth, 21),
		},
		[]string{"reconcile_type"},
	)
)

func newMetrics(host string) metrics {
	return metrics{
		host:               host,
		apiRequestDuration: apiRequestDuration,
		runLoopDuration:    runLoopDuration,
		reconcileDuration:  reconcileDuration,
	}
}

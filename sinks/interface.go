/*
Copyright 2017 Google Inc.
Copyright 2018 Alauda Inc.

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

package sinks

import (
	"event-exporter/events"
	"github.com/prometheus/client_golang/prometheus"
	api_v1 "k8s.io/api/core/v1"
	"time"
)

var (
	ReceivedEntryCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "received_entry_count",
			Help:      "Number of events, recieved by the output sink",
			Subsystem: "output_sink",
		},
		[]string{"component"},
	)

	FailedSentEntryCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name:      "successfully_sent_entry_count",
			Help:      "Number of events, successfully ingested by output sink",
			Subsystem: "output_sink",
		},
	)

	SuccessfullySentEntryCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name:      "successfully_sent_entry_count",
			Help:      "Number of events, successfully ingested by output sink",
			Subsystem: "output_sink",
		},
	)
)

const (
	defaultFlushDelay     = 5 * time.Second
	defaultMaxBufferSize  = 1000
	defaultMaxConcurrency = 1

	eventsLogName = "events"
)

type Sink interface {
	events.EventHandler

	OnList(*api_v1.EventList)

	Run(stopCh <-chan struct{})
}

type SinkCommonConf struct {
	FlushDelay     time.Duration
	MaxBufferSize  int
	MaxConcurrency int
}

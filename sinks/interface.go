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

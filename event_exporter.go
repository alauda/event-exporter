package main

import (
	"github.com/golang/glog"
	"time"

	"k8s.io/client-go/kubernetes"

	"event-exporter/sinks"
	"event-exporter/watchers"
	"sync"
)

func (e *eventExporter) Run(stopCh <-chan struct{}) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		glog.Infof("start sink")

		e.sink.Run(stopCh)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		glog.Infof("start watcher")
		e.watcher.Run(stopCh)
	}()

	<-stopCh

	wg.Wait()
}

type eventExporter struct {
	sink    sinks.Sink
	watcher watchers.Watcher
}

func newEventExporter(client kubernetes.Interface, sink sinks.Sink, resyncPeriod time.Duration) *eventExporter {
	config := &watchers.EventWatcherConfig{
		ResyncPeriod: resyncPeriod,
		Handler:      sink,
		OnList:       sink.OnList,
	}

	w := watchers.NewEventWatcher(client, config)
	return &eventExporter{
		sink:    sink,
		watcher: w,
	}
}

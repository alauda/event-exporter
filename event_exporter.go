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

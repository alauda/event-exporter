package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"event-exporter/sinks"
)

const (
	sinkNameElasticSearch = "elasticsearch"
	sinkNameKafka         = "kafka"
	sinkNameHTTPOutput    = "http"
)

var (
	resyncPeriod       = flag.Duration("resync-period", 1*time.Minute, "Reflector resync period")
	prometheusEndpoint = flag.String("prometheus-endpoint", ":80", "Endpoint on which to "+
		"expose Prometheus http handler")
	sinkName              = flag.String("sink", sinkNameElasticSearch, "Sink type to save the exported events: elasticsearch/kafka/http")
	elasticsearchEndpoint = flag.String("elasticsearch", "http://elasticsearch:9200/", "Elasticsearch endpoint")
)

func newSystemStopChannel() chan struct{} {
	ch := make(chan struct{})
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		sig := <-c
		glog.Infof("Recieved signal %s, terminating", sig.String())

		ch <- struct{}{}
	}()

	return ch
}

func newKubernetesClient() (kubernetes.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %v", err)
	}

	return kubernetes.NewForConfig(config)
}

func main() {
	flag.Set("logtostderr", "true")
	defer glog.Flush()
	flag.Parse()
	var outSink sinks.Sink
	var err error
	var k8sClient kubernetes.Interface

	if *sinkName == sinkNameElasticSearch {
		config := sinks.DefaultElasticSearchConf()
		config.Endpoint = *elasticsearchEndpoint
		outSink, err = sinks.NewElasticSearchOut(config)
		if err != nil {
			glog.Fatalf("Failed to initialize elasticsearch output: %v", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(-1)
	}

	k8sClient, err = newKubernetesClient()
	if err != nil {
		glog.Fatalf("Failed to initialize kubernetes client: %v", err)
	}

	eventExporter := newEventExporter(k8sClient, outSink, *resyncPeriod)

	// Expose the Prometheus http endpoint
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		glog.Fatalf("Prometheus monitoring failed: %v", http.ListenAndServe(*prometheusEndpoint, nil))
	}()

	stopCh := newSystemStopChannel()
	eventExporter.Run(stopCh)
}

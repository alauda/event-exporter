package sinks

import (
	"github.com/golang/glog"
	"gopkg.in/olivere/elastic.v3"
	api_v1 "k8s.io/api/core/v1"
	"time"
)

type ElasticSearchConf struct {
	Endpoint       string
	User           string
	Password       string
	FlushDelay     time.Duration
	MaxBufferSize  int
	MaxConcurrency int
}

type ElasticSearchOut struct {
	config          *ElasticSearchConf
	esClient        *elastic.Client
	beforeFirstList bool
	currentBuffer   []*api_v1.Event
	logEntryChannel chan *api_v1.Event
	// Channel for controlling how many requests are being sent at the same
	// time. It's empty initially, each request adds an object at the start
	// and takes it out upon completion. Channel's capacity is set to the
	// maximum level of parallelism, so any extra request will lock on addition.
	concurrencyChannel chan struct{}
	timer              *time.Timer
	fakeTimeChannel    chan time.Time
}

func DefaultElasticSearchConf() *ElasticSearchConf {
	return &ElasticSearchConf{
		FlushDelay:     defaultFlushDelay,
		MaxBufferSize:  defaultMaxBufferSize,
		MaxConcurrency: defaultMaxConcurrency,
	}
}

func NewElasticSearchOut(config *ElasticSearchConf) (*ElasticSearchOut, error) {
	esClient, err := elastic.NewClient(elastic.SetSniff(false),
		elastic.SetHealthcheckTimeoutStartup(10*time.Second), elastic.SetURL(config.Endpoint))
	if err != nil {
		glog.Errorf("Error create elasticsearch(%s) output %v", config.Endpoint, err)
		return nil, err
	}

	glog.Infof("NewElasticSearchOut inited.")

	return &ElasticSearchOut{
		esClient:           esClient,
		beforeFirstList:    true,
		logEntryChannel:    make(chan *api_v1.Event, config.MaxBufferSize),
		config:             config,
		currentBuffer:      []*api_v1.Event{},
		timer:              nil,
		fakeTimeChannel:    make(chan time.Time),
		concurrencyChannel: make(chan struct{}, config.MaxConcurrency),
	}, nil
}

func (es *ElasticSearchOut) OnAdd(event *api_v1.Event) {
	ReceivedEntryCount.WithLabelValues(event.Source.Component).Inc()
	glog.Infof("OnAdd %v", event)
	es.logEntryChannel <- event
}

func (es *ElasticSearchOut) OnUpdate(oldEvent *api_v1.Event, newEvent *api_v1.Event) {
	var oldCount int32
	if oldEvent != nil {
		oldCount = oldEvent.Count
	}

	if newEvent.Count != oldCount+1 {
		// Sink doesn't send a LogEntry to Stackdriver, b/c event compression might
		// indicate that part of the watch history was lost, which may result in
		// multiple events being compressed. This may create an unecessary
		// flood in Stackdriver. Also this is a perfectly valid behavior for the
		// configuration with empty backing storage.
		glog.V(2).Infof("Event count has increased by %d != 1.\n"+
			"\tOld event: %+v\n\tNew event: %+v", newEvent.Count-oldCount, oldEvent, newEvent)
	}
	glog.Infof("OnUpdate %v", newEvent)

	ReceivedEntryCount.WithLabelValues(newEvent.Source.Component).Inc()

	es.logEntryChannel <- newEvent
}

func (es *ElasticSearchOut) OnDelete(*api_v1.Event) {
	// Nothing to do here
}

func (es *ElasticSearchOut) OnList(list *api_v1.EventList) {
	// Nothing to do else
	glog.Infof("OnList %v", list)
	if es.beforeFirstList {
		es.beforeFirstList = false
	}
}

func (es *ElasticSearchOut) Run(stopCh <-chan struct{}) {
	glog.Info("Starting Elasticsearch sink")
	for {
		select {
		case entry := <-es.logEntryChannel:
			es.currentBuffer = append(es.currentBuffer, entry)
			if len(es.currentBuffer) >= es.config.MaxBufferSize {
				es.flushBuffer()
			} else if len(es.currentBuffer) == 1 {
				es.setTimer()
			}
			break
		case <-es.getTimerChannel():
			es.flushBuffer()
			break
		case <-stopCh:
			glog.Info("Elasticsearch sink recieved stop signal, waiting for all requests to finish")
			glog.Info("All requests to Elasticsearch finished, exiting Elasticsearch sink")
			return
		}
	}
}

func (es *ElasticSearchOut) flushBuffer() {
	entries := es.currentBuffer
	es.currentBuffer = nil
	es.concurrencyChannel <- struct{}{}
	go es.sendEntries(entries)
}
func (es *ElasticSearchOut) sendEntries(entries []*api_v1.Event) {
	glog.V(4).Infof("Sending %d entries to Elasticsearch", len(entries))

	bulkRequest := es.esClient.Bulk()

	for _, event := range entries {
		glog.Infof("Orig obj: %v", event.InvolvedObject)
		newIndex := elastic.NewBulkIndexRequest().Index(eventsLogName).Type(eventsLogName).Id(string(event.ObjectMeta.UID)).Doc(event)
		bulkRequest = bulkRequest.Add(newIndex)
	}

	_, err := bulkRequest.Do()
	if err != nil {
		glog.Errorf("save events error: %v", err)
	}

	SuccessfullySentEntryCount.Add(float64(len(entries)))

	<-es.concurrencyChannel

	glog.V(4).Infof("Successfully sent %d entries to Elasticsearch", len(entries))
}

func (es *ElasticSearchOut) setTimer() {
	if es.timer == nil {
		es.timer = time.NewTimer(es.config.FlushDelay)
	} else {
		es.timer.Reset(es.config.FlushDelay)
	}
}

func (es *ElasticSearchOut) getTimerChannel() <-chan time.Time {
	if es.timer == nil {
		return es.fakeTimeChannel
	}
	return es.timer.C
}

package sinks

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"io"
	api_v1 "k8s.io/api/core/v1"
	"net/http"
	"time"
)

const (
	authMethodToken        = "token"
	authMethodBasic        = "basice"
	defaultTimeout         = 30 * time.Second
	contentEncodingGzip    = "gzip"
	defaultContentEncoding = contentEncodingGzip
)

type HTTPConf struct {
	SinkCommonConf
	Endpoint        *string
	Auth            *string // Basic or token
	Username        *string
	Password        *string
	Token           *string
	Timeout         time.Duration
	ContentEncoding string
}

type HTTPSink struct {
	config          *HTTPConf
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

func DefaultHTTPConf() *HTTPConf {
	return &HTTPConf{
		Timeout:         defaultTimeout,
		ContentEncoding: defaultContentEncoding,
		SinkCommonConf: SinkCommonConf{
			FlushDelay:     defaultFlushDelay,
			MaxBufferSize:  defaultMaxBufferSize,
			MaxConcurrency: defaultMaxConcurrency,
		},
	}
}

func NewHTTPSink(config *HTTPConf) (*HTTPSink, error) {

	return &HTTPSink{
		beforeFirstList:    true,
		logEntryChannel:    make(chan *api_v1.Event, config.MaxBufferSize),
		config:             config,
		currentBuffer:      []*api_v1.Event{},
		timer:              nil,
		fakeTimeChannel:    make(chan time.Time),
		concurrencyChannel: make(chan struct{}, config.MaxConcurrency),
	}, nil
}

func (h *HTTPSink) OnAdd(event *api_v1.Event) {
	ReceivedEntryCount.WithLabelValues(event.Source.Component).Inc()
	glog.Infof("OnAdd %v", event)
	h.logEntryChannel <- event
}

func (h *HTTPSink) OnUpdate(oldEvent *api_v1.Event, newEvent *api_v1.Event) {
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

	h.logEntryChannel <- newEvent
}

func (h *HTTPSink) OnDelete(*api_v1.Event) {
	// Nothing to do here
}

func (h *HTTPSink) OnList(list *api_v1.EventList) {
	// Nothing to do else
	glog.Infof("OnList %v", list)
	if h.beforeFirstList {
		h.beforeFirstList = false
	}
}

func (h *HTTPSink) Run(stopCh <-chan struct{}) {
	glog.Info("Starting Elasticsearch sink")
	for {
		select {
		case entry := <-h.logEntryChannel:
			h.currentBuffer = append(h.currentBuffer, entry)
			if len(h.currentBuffer) >= h.config.MaxBufferSize {
				h.flushBuffer()
			} else if len(h.currentBuffer) == 1 {
				h.setTimer()
			}
			break
		case <-h.getTimerChannel():
			h.flushBuffer()
			break
		case <-stopCh:
			glog.Info("Elasticsearch sink recieved stop signal, waiting for all requests to finish")
			glog.Info("All requests to Elasticsearch finished, exiting Elasticsearch sink")
			return
		}
	}
}

func (h *HTTPSink) flushBuffer() {
	entries := h.currentBuffer
	h.currentBuffer = nil
	h.concurrencyChannel <- struct{}{}
	go h.sendEntries(entries)
}
func (h *HTTPSink) sendEntries(entries []*api_v1.Event) {
	glog.V(4).Infof("Sending %d entries to Elasticsearch", len(entries))

	if err := doHttpRequest(h.config, entries); err != nil {
		// TODO how to recovery?
		FailedSentEntryCount.Add(float64(len(entries)))
	} else {
		SuccessfullySentEntryCount.Add(float64(len(entries)))
	}

	<-h.concurrencyChannel

	glog.V(4).Infof("Successfully sent %d entries to Elasticsearch", len(entries))
}

func (h *HTTPSink) setTimer() {
	if h.timer == nil {
		h.timer = time.NewTimer(h.config.FlushDelay)
	} else {
		h.timer.Reset(h.config.FlushDelay)
	}
}

func (h *HTTPSink) getTimerChannel() <-chan time.Time {
	if h.timer == nil {
		return h.fakeTimeChannel
	}
	return h.timer.C
}

func encodeData(v interface{}) (*bytes.Buffer, error) {
	param := bytes.NewBuffer(nil)
	j, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	if _, err := param.Write(j); err != nil {
		return nil, err
	}
	return param, nil
}

func setHeaders(req *http.Request, config *HTTPConf) error {

	if *config.Auth == authMethodBasic {
		// Basic auth
		req.SetBasicAuth(*config.Username, *config.Password)
	} else if *config.Auth == authMethodToken {
		// config.Token should contains Bearer or other method, not the token value only
		req.Header.Set("Authorization", *config.Token)
	} else {
		return fmt.Errorf("Auth method error: %s not supported yet", *config.Auth)
	}

	req.Header.Set("Context-Type", "application/json")
	req.Header.Set("User-Agent", "alauda/event-exporter/1.0")

	return nil
}

func doHttpRequest(config *HTTPConf, data interface{}) error {

	// Convert to json string
	params, err := encodeData(data)
	if err != nil {
		return err
	}

	// Compress data
	if config.ContentEncoding == contentEncodingGzip {
		var b bytes.Buffer
		w := zlib.NewWriter(&b)
		defer w.Close()
		if _, err := io.Copy(w, params); err != nil {
			return err
		}
		params = &b
	}

	// Create req
	req, err := http.NewRequest("POST", *config.Endpoint, params)

	if err != nil {
		return err
	}

	// Set headers
	if err := setHeaders(req, config); err != nil {
		return err
	}

	// Issue POST request
	client := &http.Client{Timeout: config.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check return code
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Not a expected: %d.", resp.StatusCode)
	}
	return nil
}

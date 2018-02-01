package watchers

import (
	"github.com/golang/glog"
	api_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"time"

	"event-exporter/events"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	// Since events live in the apiserver only for 1 hour, we have to remove
	// old objects to avoid memory leaks. If TTL is exactly 1 hour, race
	// can occur in case of the event being updated right before the end of
	// the hour, since it takes some time to deliver this event via watch.
	// 2 hours ought to be enough for anybody.
	eventStorageTTL = 2 * time.Hour
)

// Watcher is an interface of the generic proactive API watcher.
type Watcher interface {
	Run(stopCh <-chan struct{})
}

type watcher struct {
	reflector *cache.Reflector
}

func (w *watcher) Run(stopCh <-chan struct{}) {
	glog.Infof("start reflector")

	w.reflector.Run(stopCh)
	<-stopCh
}

// OnListFunc represent an action on the initial list of object received
// from the Kubernetes API server before starting watching for the updates.
type OnListFunc func(*api_v1.EventList)

// WatcherConfig represents the configuration of the Kubernetes API watcher.
type EventWatcherConfig struct {
	ResyncPeriod time.Duration
	OnList       OnListFunc
	Handler      events.EventHandler
}

// NewEventWatcher create a new watcher that only watches the events resource.
func NewEventWatcher(client kubernetes.Interface, config *EventWatcherConfig) Watcher {
	listerWatcher := &cache.ListWatch{
		ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
			glog.Infof("ListFunc %v", options)
			list, err := client.CoreV1().Events(meta_v1.NamespaceAll).List(options)
			glog.Infof("got list %v", list)
			if err == nil {
				config.OnList(list)
			}
			return list, err
		},
		WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
			glog.Infof("WatchFunc %v", options)
			return client.CoreV1().Events(meta_v1.NamespaceAll).Watch(options)
		},
	}

	storeConfig := &WatcherStoreConfig{
		KeyFunc:     cache.DeletionHandlingMetaNamespaceKeyFunc,
		Handler:     events.NewEventHandlerWrapper(config.Handler),
		StorageType: TTLStorage,
		StorageTTL:  eventStorageTTL,
	}

	return &watcher{
		reflector: cache.NewReflector(
			listerWatcher,
			&api_v1.Event{},
			newWatcherStore(storeConfig),
			config.ResyncPeriod,
		),
	}
}

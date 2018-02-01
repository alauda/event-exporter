package events

import (
	"github.com/golang/glog"

	api_v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

type EventHandler interface {
	OnAdd(*api_v1.Event)
	OnUpdate(*api_v1.Event, *api_v1.Event)
	OnDelete(*api_v1.Event)
}

type eventHandlerWrapper struct {
	handler EventHandler
}

func NewEventHandlerWrapper(handler EventHandler) *eventHandlerWrapper {
	return &eventHandlerWrapper{
		handler: handler,
	}
}

func (c *eventHandlerWrapper) OnAdd(obj interface{}) {
	if event, ok := c.convert(obj); ok {
		c.handler.OnAdd(event)
	}
}

func (c *eventHandlerWrapper) OnUpdate(oldObj interface{}, newObj interface{}) {
	oldEvent, oldOk := c.convert(oldObj)
	newEvent, newOk := c.convert(newObj)
	if newOk && (oldObj == nil || oldOk) {
		c.handler.OnUpdate(oldEvent, newEvent)
	}
}

func (c *eventHandlerWrapper) OnDelete(obj interface{}) {

	event, ok := obj.(*api_v1.Event)

	// When a delete is dropped, the relist will notice a pod in the store not
	// in the list, leading to the insertion of a tombstone object which contains
	// the deleted key/value. Note that this value might be stale.
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			glog.V(2).Infof("Object is neither event nor tombstone: %+v", obj)
			return
		}
		event, ok = tombstone.Obj.(*api_v1.Event)
		if !ok {
			glog.V(2).Infof("Tombstone contains object that is not a pod: %+v", obj)
			return
		}
	}

	c.handler.OnDelete(event)
}

func (c *eventHandlerWrapper) convert(obj interface{}) (*api_v1.Event, bool) {
	if event, ok := obj.(*api_v1.Event); ok {
		return event, true
	}
	glog.V(2).Infof("Event watch handler recieved not event, but %+v", obj)
	return nil, false
}

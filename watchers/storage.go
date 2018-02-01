package watchers

import (
	"time"

	"k8s.io/client-go/tools/cache"
)

// StorageType defines what storage should be used as a cache for the watcher.
type StorageType int

const (
	// SimpleStorage storage type indicates thread-safe map as backing storage.
	SimpleStorage StorageType = iota
	// TTLStorage storage type indicates storage with expiration. When this
	// type of storage is used, TTL should be specified.
	TTLStorage
)

// WatcherStoreConfig represents the configuration of the storage backing the watcher.
type WatcherStoreConfig struct {
	KeyFunc     cache.KeyFunc
	Handler     cache.ResourceEventHandler
	StorageType StorageType
	StorageTTL  time.Duration
}

type watcherStore struct {
	cache.Store

	handler cache.ResourceEventHandler
}

func (s *watcherStore) Add(obj interface{}) error {
	if err := s.Store.Add(obj); err != nil {
		return err
	}
	s.handler.OnAdd(obj)
	return nil
}

func (s *watcherStore) Update(obj interface{}) error {
	oldObj, ok, err := s.Store.Get(obj)
	if err != nil {
		return err
	}
	if !ok {
		oldObj = nil
	}

	if err = s.Store.Update(obj); err != nil {
		return err
	}
	s.handler.OnUpdate(oldObj, obj)
	return nil
}

func (s *watcherStore) Delete(obj interface{}) error {
	if err := s.Store.Delete(obj); err != nil {
		return err
	}
	s.handler.OnDelete(obj)
	return nil
}

func newWatcherStore(config *WatcherStoreConfig) *watcherStore {
	var cacheStorage cache.Store
	switch config.StorageType {
	case TTLStorage:
		cacheStorage = cache.NewTTLStore(config.KeyFunc, config.StorageTTL)
		break
	case SimpleStorage:
	default:
		cacheStorage = cache.NewStore(config.KeyFunc)
		break
	}

	return &watcherStore{
		Store:   cacheStorage,
		handler: config.Handler,
	}
}

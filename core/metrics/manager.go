package metrics

import (
	"github.com/byteplus-sdk/sdk-go/core/logs"
	"sync"
	"sync/atomic"
	"time"
)

const (
	//default expire interval of each counter/timer/store, expired metrics will be cleaned
	defaultTTL = 100 * time.Second
	//interval of flushing all cache metrics
	defaultFlushInterval = 15 * time.Second
)

var managerCache = &InstanceCache{
	instanceMap:     make(map[string]interface{}),
	instanceBuilder: newManager,
	lock:            &sync.Mutex{},
}

type Manager struct {
	prefix        string
	ttl           time.Duration
	locks         map[metricsType]*sync.RWMutex
	flushInterval time.Duration
	metricsMaps   map[metricsType]map[string]ExpirableMetrics
	stopped       int32
}

// GetManager return instance of client according metrics prefix
func GetManager(prefix string) *Manager {
	return managerCache.GetInstanceByName(prefix).(*Manager)
}

func newManager(prefix string) interface{} {
	return newManagerWithParams(prefix, defaultTTL, defaultFlushInterval)
}

//ttl should be longer than flushInterval
func newManagerWithParams(prefix string, ttl time.Duration, flushInterval time.Duration) *Manager {
	manager := &Manager{
		prefix: prefix,
		ttl:    ttl,
		locks: map[metricsType]*sync.RWMutex{
			metricsTypeStore:   &sync.RWMutex{},
			metricsTypeCounter: &sync.RWMutex{},
			metricsTypeTimer:   &sync.RWMutex{},
		},
		flushInterval: flushInterval,
		metricsMaps: map[metricsType]map[string]ExpirableMetrics{
			metricsTypeStore:   make(map[string]ExpirableMetrics, 256),
			metricsTypeCounter: make(map[string]ExpirableMetrics, 256),
			metricsTypeTimer:   make(map[string]ExpirableMetrics, 256),
		},
		stopped: 0,
	}
	manager.start()
	return manager
}

func (c *Manager) start() {
	atomic.StoreInt32(&c.stopped, 0)
	// Regularly report metrics, one thread for each type
	for metricsType := range c.metricsMaps {
		metricsType := metricsType
		go func() {
			ticker := time.NewTicker(c.flushInterval)
			for {
				if c.isStopped() {
					return
				}
				<-ticker.C
				c.reportMetrics(metricsType)
			}
		}()
	}
	// Regularly clean up overdue metrics
	go func() {
		ticker := time.NewTicker(c.ttl)
		for {
			if c.isStopped() {
				return
			}
			<-ticker.C
			c.tidy()
		}
	}()
}

func (c *Manager) isStopped() bool {
	return atomic.LoadInt32(&c.stopped) == 1
}

func (c *Manager) Stop() {
	atomic.StoreInt32(&c.stopped, 1)
}

func (c *Manager) reportMetrics(metricsType metricsType) {
	c.locks[metricsType].RLock()
	defer c.locks[metricsType].RUnlock()
	for _, metrics := range c.metricsMaps[metricsType] { //read map
		// report stores
		metrics.flush()
	}
}

func (c *Manager) tidy() {
	// clean expired metrics
	for metricsType, metricsMap := range c.metricsMaps {
		expiredMetrics := make([]ExpirableMetrics, 0)
		for _, metrics := range metricsMap {
			if metrics.isExpired() {
				expiredMetrics = append(expiredMetrics, metrics)
			}
		}
		if len(expiredMetrics) != 0 {
			c.locks[metricsType].Lock()
			for _, metrics := range expiredMetrics {
				delete(metricsMap, metrics.getName())
				if IsEnablePrintLog() {
					logs.Info("remove expired metrics %+v", metrics.getName())
				}
			}
			c.locks[metricsType].Unlock()
		}
	}
}

func (c *Manager) emitCounter(name string, tags map[string]string, value float64) {
	c.getOrAddMetrics(metricsTypeCounter, c.prefix+"."+name, nil).emit(value, tags)
}

func (c *Manager) emitTimer(name string, tags map[string]string, value float64) {
	c.getOrAddMetrics(metricsTypeTimer, c.prefix+"."+name, tags).emit(value, nil)
}

func (c *Manager) emitStore(name string, tags map[string]string, value float64) {
	c.getOrAddMetrics(metricsTypeStore, c.prefix+"."+name, nil).emit(value, tags)
}

func (c *Manager) getOrAddMetrics(metricsType metricsType, name string, tagKvs map[string]string) ExpirableMetrics {
	tagString := ""
	if len(tagKvs) != 0 {
		tagString = processTags(tagKvs)
	}
	metricsKey := name + tagString
	metrics, exist := c.metricsMaps[metricsType][metricsKey]
	if !exist {
		c.locks[metricsType].Lock()
		defer c.locks[metricsType].Unlock()
		metrics, exist := c.metricsMaps[metricsType][metricsKey]
		if !exist {
			metrics = c.buildMetrics(metricsType, name, tagString)
			metrics.updateExpireTime(c.ttl)
			c.metricsMaps[metricsType][metricsKey] = metrics
			return metrics
		}
	}
	return metrics
}

func (c *Manager) buildMetrics(metricsType metricsType, name string, tagString string) ExpirableMetrics {
	switch metricsType {
	case metricsTypeStore:
		return NewStoreWithFlushTime(name, c.flushInterval)
	case metricsTypeCounter:
		return NewCounterWithFlushTime(name, c.flushInterval)
	case metricsTypeTimer:
		return NewTimerWithFlushTime(name, tagString, c.flushInterval)
	}
	return nil
}

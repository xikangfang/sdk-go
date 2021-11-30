package metrics

import (
	"github.com/byteplus-sdk/sdk-go/core/logs"
	"sync"
	"sync/atomic"
	"time"
)

var (
	cliLocker      = &sync.Mutex{}
	clientCache    = make(map[string]*Client)
)

const (
	//default expire interval of each counter/timer/store, expired metrics will be cleaned
	defaultTTL = 100 * time.Second
	//interval of flushing all cache metrics
	defaultFlushInterval = 15 * time.Second
)

type Client struct {
	prefix         string
	ttl            time.Duration
	locker         *sync.Mutex
	flushInterval  time.Duration
	storeMetrics   map[string]*Store
	counterMetrics map[string]*Counter
	timerMetrics   map[string]*Timer
	ticker         *time.Ticker
	stopped        int32
}

// GetClientByPrefix return instance of client according metrics prefix
func GetClientByPrefix(prefix string) *Client {
	cliLocker.Lock()
	defer cliLocker.Unlock()
	if cli, ok := clientCache[prefix]; ok {
		return cli
	}
	cli := newClientWithParams(prefix, defaultTTL, defaultFlushInterval)
	clientCache[prefix] = cli
	cli.start()
	return cli
}

func newClient() *Client {
	return newClientWithParams(defaultMetricsPrefix, defaultTTL, defaultFlushInterval)
}

func newClientWithParams(prefix string, ttl time.Duration, flushInterval time.Duration) *Client {
	return &Client{
		prefix:         prefix,
		ttl:            ttl,
		locker:         &sync.Mutex{},
		flushInterval:  flushInterval,
		storeMetrics:   make(map[string]*Store, 256),
		counterMetrics: make(map[string]*Counter, 256),
		timerMetrics:   make(map[string]*Timer, 256),
		ticker:         time.NewTicker(ttl),
		stopped:        -1,
	}
}

func (c *Client) start() {
	atomic.StoreInt32(&c.stopped, 0)
	go func() {
		for {
			if c.isStopped() {
				return
			}
			<-c.ticker.C
			c.tidy()
		}
	}()
}

func (c *Client) isStopped() bool {
	return atomic.LoadInt32(&c.stopped) == 1
}

func (c *Client) Stop() {
	atomic.StoreInt32(&c.stopped, 1)
}

func (c *Client) tidy() {
	// clean expired store
	expiredStores := make([]*Store, 0)
	for _, store := range c.storeMetrics {
		if store.expirableMetrics.isExpired() {
			expiredStores = append(expiredStores, store)
		}
	}
	if len(expiredStores) != 0 {
		c.locker.Lock()
		for _, store := range expiredStores {
			store.close()
			delete(c.storeMetrics, store.name)
			if IsEnablePrintLog() {
				logs.Info("remove expired store metrics %+v", store.name)
			}
		}
		c.locker.Unlock()
	}

	// clean expired counter
	expiredCounters := make([]*Counter, 0)
	for _, counter := range c.counterMetrics {
		if counter.expirableMetrics.isExpired() {
			expiredCounters = append(expiredCounters, counter)
		}
	}
	if len(expiredCounters) != 0 {
		c.locker.Lock()
		for _, counter := range expiredCounters {
			counter.stop()
			delete(c.counterMetrics, counter.name)
			if IsEnablePrintLog() {
				logs.Info("remove expired counter metrics %+v", counter.name)
			}
		}
		c.locker.Unlock()
	}

	// clean expired timer
	expiredTimers := make([]*Timer, 0)
	for _, timer := range c.timerMetrics {
		if timer.expirableMetrics.isExpired() {
			expiredTimers = append(expiredTimers, timer)
		}
	}
	if len(expiredTimers) != 0 {
		c.locker.Lock()
		for _, timer := range expiredTimers {
			timer.close()
			delete(c.timerMetrics, timer.name)
			if IsEnablePrintLog() {
				logs.Info("remove expired timer metrics %+v", timer.name)
			}
		}
		c.locker.Unlock()
	}
}

func (c *Client) emitCounter(name string, tags map[string]string, value float64) {
	c.getOrAddTsCounter(c.prefix+"."+name).emit(tags, value)
}

func (c *Client) emitTimer(name string, tags map[string]string, value float64) {
	c.getOrAddTsTimer(c.prefix+"."+name, tags).emit(value)
}

func (c *Client) emitStore(name string, tags map[string]string, value float64) {
	c.getOrAddTsStore(c.prefix+"."+name).emit(tags, value)
}

func (c *Client) getOrAddTsStore(name string) *Store {
	store, exist := c.storeMetrics[name]
	if !exist {
		c.locker.Lock()
		defer c.locker.Unlock()
		store, exist := c.storeMetrics[name]
		if !exist {
			store = NewStoreWithFlushTime(name, c.flushInterval)
			store.expirableMetrics.updateExpireTime(c.ttl)
			c.storeMetrics[name] = store
			store.start()
			return store
		}
	}
	return store
}

func (c *Client) getOrAddTsCounter(name string) *Counter {
	counter, exist := c.counterMetrics[name]
	if !exist {
		c.locker.Lock()
		defer c.locker.Unlock()
		counter, exist := c.counterMetrics[name]
		if !exist {
			counter = NewCounterWithFlushTime(name, c.flushInterval)
			counter.expirableMetrics.updateExpireTime(c.ttl)
			c.counterMetrics[name] = counter
			counter.start()
			return counter
		}
	}
	return counter
}

func (c *Client) getOrAddTsTimer(name string, tagKvs map[string]string) *Timer {
	tags := processTags(tagKvs)
	nameWithTag := name + tags
	timer, exist := c.timerMetrics[nameWithTag]
	if !exist {
		c.locker.Lock()
		defer c.locker.Unlock()
		timer, exist := c.timerMetrics[nameWithTag]
		if !exist {
			timer = NewTimerWithFlushTime(name, tags, c.flushInterval)
			timer.expirableMetrics.updateExpireTime(c.ttl)
			c.timerMetrics[nameWithTag] = timer
			timer.start()
			return timer
		}
	}
	return timer
}

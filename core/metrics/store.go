package metrics

import (
	"fmt"
	"github.com/byteplus-sdk/sdk-go/core/logs"
	"sync"
	"sync/atomic"
	"time"
)

type Store struct {
	name             string
	lock             *sync.Mutex
	expirableMetrics *ExpirableMetrics
	queue            chan *Item
	valueMap         map[Item]*MetricsRequest
	httpCli          *HttpClient
	ticker           *time.Ticker
	closed           int32
}

func NewStore(name string) *Store {
	return NewStoreWithFlushTime(name, defaultFlushInterval)
}

func NewStoreWithFlushTime(name string, flushInterval time.Duration) *Store {
	c := &Store{
		name:             name,
		expirableMetrics: NewExpirableMetrics(),
		lock:             &sync.Mutex{},
		queue:            make(chan *Item, maxQueueSize),
		valueMap:         make(map[Item]*MetricsRequest),
		httpCli:          NewHttpClient(fmt.Sprintf(otherUrlFormat, metricsDomain)),
		ticker:           time.NewTicker(flushInterval),
		closed:           0,
	}
	return c
}

func (c *Store) emit(tags map[string]string, value float64) {
	tag := processTags(tags)
	item := NewItem(tag, value)
	select {
	case c.queue <- item:
	default:
		if IsEnablePrintLog() {
			logs.Warn("metrics emit too fast, exceed max queue size(%d)", maxQueueSize)
		}
	}
}

func (c *Store) flush() {
	defer func() {
		if err := recover(); err != nil {
			logs.Error("exec store err: %v", err)
		}
	}()
	for size := 0; size < maxFlashSize && len(c.queue) != 0; size++ {
		item := <-c.queue
		if req, ok := c.valueMap[*item]; ok {
			req.Value = item.value
		} else {
			metricsRequest := &MetricsRequest{
				MetricsName: c.name,
				Value:       item.value,
				TagKvs:      recoverTags(item.tags),
			}
			c.valueMap[*item] = metricsRequest
		}
	}
	metricsRequests := make([]*MetricsRequest, 0, len(c.valueMap))
	if len(c.valueMap) != 0 {
		timestamp := time.Now().Unix()
		for item, metricsRequest := range c.valueMap {
			metricsRequest.TimeStamp = timestamp
			metricsRequests = append(metricsRequests, metricsRequest)
			delete(c.valueMap, item)
			if IsEnablePrintLog() {
				logs.Info("remove counter key: %+v", item)
			}
		}
		if success := c.httpCli.emit(metricsRequests); !success {
			logs.Error("exec counter fail")
		}
	}
}

func (c *Store) start() {
	atomic.StoreInt32(&c.closed, 0)
	go func() {
		for {
			if c.isClosed() {
				return
			}
			<-c.ticker.C
			c.flush()
		}
	}()
}

func (c *Store) isClosed() bool {
	return atomic.LoadInt32(&c.closed) == 1
}

func (c *Store) close() {
	atomic.StoreInt32(&c.closed, 1) //todo:是否需要关闭channel
}

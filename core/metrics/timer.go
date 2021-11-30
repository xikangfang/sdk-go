package metrics

import (
	"fmt"
	"github.com/byteplus-sdk/sdk-go/core/logs"
	gm "github.com/rcrowley/go-metrics"
	"sync"
	"sync/atomic"
	"time"
)

const reservoirSize = 65536

type Timer struct {
	name             string
	tagKvs           map[string]string
	lock             *sync.Mutex
	expirableMetrics *ExpirableMetrics
	queue            chan float64
	reservoir        gm.Sample
	httpCli          *HttpClient
	ticker           *time.Ticker
	closed           int32
}

func NewTimer(name string) *Counter {
	return NewCounterWithFlushTime(name, defaultFlushInterval)
}

func NewTimerWithFlushTime(name string, tags string, flushInterval time.Duration) *Timer {
	c := &Timer{
		name:             name,
		tagKvs:           recoverTags(tags),
		expirableMetrics: NewExpirableMetrics(),
		lock:             &sync.Mutex{},
		queue:            make(chan float64, maxQueueSize),
		reservoir:        gm.NewUniformSample(reservoirSize),
		httpCli:          NewHttpClient(fmt.Sprintf(otherUrlFormat, metricsDomain)),
		ticker:           time.NewTicker(flushInterval),
		closed:           0,
	}
	return c
}

func (c *Timer) emit(value float64) {
	select {
	case c.queue <- value:
	default:
		if IsEnablePrintLog() {
			logs.Warn("metrics emit too fast, exceed max queue size(%d)", maxQueueSize)
		}
	}
}

func (c *Timer) flush() {
	defer func() {
		if err := recover(); err != nil {
			logs.Error("exec timer err: %v", err)
		}
	}()
	var size int64
	for size = 0; size < maxFlashSize && len(c.queue) != 0; size++ {
		item := <-c.queue
		c.reservoir.Update(int64(item))
	}
	snapshot := c.reservoir.Snapshot()
	metricsRequests := c.buildMetricRequests(snapshot, size)
	if success := c.httpCli.emit(metricsRequests); !success {
		logs.Error("exec timer fail")
	}

}

func (c *Timer) buildMetricRequests(snapshot gm.Sample, size int64) []*MetricsRequest {
	metricsRequests := make([]*MetricsRequest, 0)
	timestamp := time.Now().Unix()

	//count
	countRequest := &MetricsRequest{}
	countRequest.MetricsName = c.name + "." + "count"
	countRequest.TimeStamp = timestamp
	countRequest.TagKvs = StringMapClone(c.tagKvs)
	countRequest.Value = float64(size)
	metricsRequests = append(metricsRequests, countRequest)

	//max
	maxRequest := &MetricsRequest{}
	maxRequest.MetricsName = c.name + "." + "max"
	maxRequest.TimeStamp = timestamp
	maxRequest.TagKvs = StringMapClone(c.tagKvs)
	maxRequest.Value = float64(snapshot.Max())
	metricsRequests = append(metricsRequests, maxRequest)

	//min
	minRequest := &MetricsRequest{}
	minRequest.MetricsName = c.name + "." + "min"
	minRequest.TimeStamp = timestamp
	minRequest.TagKvs = StringMapClone(c.tagKvs)
	minRequest.Value = float64(snapshot.Min())
	metricsRequests = append(metricsRequests, minRequest)

	//avg
	avgRequest := &MetricsRequest{}
	avgRequest.MetricsName = c.name + "." + "avg"
	avgRequest.TimeStamp = timestamp
	avgRequest.TagKvs = StringMapClone(c.tagKvs)
	avgRequest.Value = snapshot.Mean()
	metricsRequests = append(metricsRequests, avgRequest)

	//median
	medianRequest := &MetricsRequest{}
	medianRequest.MetricsName = c.name + "." + "median"
	medianRequest.TimeStamp = timestamp
	medianRequest.TagKvs = StringMapClone(c.tagKvs)
	medianRequest.Value = snapshot.Percentile(0.5)
	metricsRequests = append(metricsRequests, medianRequest)

	//pc75
	pc75Request := &MetricsRequest{}
	pc75Request.MetricsName = c.name + "." + "pct75"
	pc75Request.TimeStamp = timestamp
	pc75Request.TagKvs = StringMapClone(c.tagKvs)
	pc75Request.Value = snapshot.Percentile(0.75)
	metricsRequests = append(metricsRequests, pc75Request)

	//pc90
	pc90Request := &MetricsRequest{}
	pc90Request.MetricsName = c.name + "." + "pct90"
	pc90Request.TimeStamp = timestamp
	pc90Request.TagKvs = StringMapClone(c.tagKvs)
	pc90Request.Value = snapshot.Percentile(0.90)
	metricsRequests = append(metricsRequests, pc90Request)

	//pc95
	pc95Request := &MetricsRequest{}
	pc95Request.MetricsName = c.name + "." + "pct95"
	pc95Request.TimeStamp = timestamp
	pc95Request.TagKvs = StringMapClone(c.tagKvs)
	pc95Request.Value = snapshot.Percentile(0.95)
	metricsRequests = append(metricsRequests, pc95Request)

	//pc99
	pc99Request := &MetricsRequest{}
	pc99Request.MetricsName = c.name + "." + "pct99"
	pc99Request.TimeStamp = timestamp
	pc99Request.TagKvs = StringMapClone(c.tagKvs)
	pc99Request.Value = snapshot.Percentile(0.99)
	metricsRequests = append(metricsRequests, pc99Request)

	//pc990
	pc999Request := &MetricsRequest{}
	pc999Request.MetricsName = c.name + "." + "pct999"
	pc999Request.TimeStamp = timestamp
	pc999Request.TagKvs = StringMapClone(c.tagKvs)
	pc999Request.Value = snapshot.Percentile(0.999)
	metricsRequests = append(metricsRequests, pc999Request)

	return metricsRequests
}

func (c *Timer) start() {
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

func (c *Timer) isClosed() bool {
	return atomic.LoadInt32(&c.closed) == 1
}

func (c *Timer) close() {
	atomic.StoreInt32(&c.closed, 1) //todo:是否需要关闭channel
}

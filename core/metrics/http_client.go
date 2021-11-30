package metrics

import (
	"encoding/json"
	"github.com/byteplus-sdk/sdk-go/core/logs"
	"github.com/valyala/fasthttp"
	"time"
)

const (
	maxTryTimes = 2
	defaultHttpTimeout = 800 * time.Millisecond
)

var httpCli = &fasthttp.Client{}

type MetricsRequest struct {
	MetricsName string            `json:"metric"`
	TagKvs      map[string]string `json:"tags"`
	Value       float64           `json:"value"`
	TimeStamp   int64             `json:"timestamp"`
}

type HttpClient struct {
	url     string
	timeout time.Duration
}

func NewHttpClient(url string) *HttpClient {
	return &HttpClient{
		url:     url,
		timeout: defaultHttpTimeout,
	}
}

// send httpRequest to metrics server
func (h *HttpClient) send(request *fasthttp.Request) bool {
	defer func() {
		fasthttp.ReleaseRequest(request)
	}()
	var err error
	var response *fasthttp.Response
	for i := 0; i < maxTryTimes; i++ {
		response = fasthttp.AcquireResponse()
		if h.timeout > 0 {
			err = httpCli.DoTimeout(request, response, h.timeout)
		} else {
			err = httpCli.Do(request, response)
		}
		if err == nil && response.StatusCode() == fasthttp.StatusOK {
			if IsEnablePrintLog() {
				logs.Debug("success reporting metrics request:%+v", request)
			}
			fasthttp.ReleaseResponse(response)
			return true
		}
	}
	logs.Error("do http request occur error:%+v, request:%+v, response:%+v, url:%s",
		err, request.String(), response, h.url)
	fasthttp.ReleaseResponse(response)
	return false
}

func (h *HttpClient) emit(metricRequest []*MetricsRequest) bool {
	request, err := h.buildMetricsRequest(metricRequest)
	if err != nil {
		logs.Error("[Metrics-SDk] build metrics error:%s", err.Error())
	}
	return h.send(request)
}

func (h *HttpClient) buildMetricsRequest(metricRequests []*MetricsRequest) (*fasthttp.Request, error) {
	request := fasthttp.AcquireRequest()
	request.Header.SetMethod(fasthttp.MethodPost)
	request.SetRequestURI(h.url)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	body, err := json.Marshal(metricRequests)
	if err != nil {
		return nil, err
	}
	request.SetBodyRaw(body)
	return request, nil
}

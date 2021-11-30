package metrics

const (
	defaultMetricsDomain = "bot.snssdk.com"
	defaultMetricsPrefix = "byteplus.rec.sdk"

	counterUrlFormat = "http://%s/api/counter"
	otherUrlFormat   = "http://%s/api/put"

	maxFlashSize = 65536 * 2
)

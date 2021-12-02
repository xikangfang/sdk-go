package metrics

import "time"

const defaultExpireTime = 12 * time.Hour

type ExpirableMetrics interface {
	getName() string
	isExpired() bool
	updateExpireTime(ttl time.Duration)
	flush()
	emit(float64, map[string]string)
}

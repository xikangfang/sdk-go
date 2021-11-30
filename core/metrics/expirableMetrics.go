package metrics

import "time"

const defaultExpireTime = 12 * time.Hour

type ExpirableMetrics struct {
	expireTime time.Time //expireTime in ms
}

func NewExpirableMetrics() *ExpirableMetrics {
	return &ExpirableMetrics{
		expireTime: time.Now().Add(defaultExpireTime),
	}
}

func (m *ExpirableMetrics) isExpired() bool {
	return time.Now().After(m.expireTime)
}

func (m *ExpirableMetrics) updateExpireTime(ttl time.Duration) {
	if ttl > 0 {
		m.expireTime = time.Now().Add(ttl)
	}
}

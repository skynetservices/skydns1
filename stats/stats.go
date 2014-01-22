package stats

import (
	"github.com/rcrowley/go-metrics"
)

var (
	ExpiredCount       metrics.Counter
	RequestCount       metrics.Counter
	AddServiceCount    metrics.Counter
	UpdateTTLCount     metrics.Counter
	GetServiceCount    metrics.Counter
	RemoveServiceCount metrics.Counter
)

func init() {
	ExpiredCount = metrics.NewCounter()
	metrics.Register("skydns-expired-entries", ExpiredCount)

	RequestCount = metrics.NewCounter()
	metrics.Register("skydns-requests", RequestCount)

	AddServiceCount = metrics.NewCounter()
	metrics.Register("skydns-add-service-requests", AddServiceCount)

	UpdateTTLCount = metrics.NewCounter()
	metrics.Register("skydns-update-ttl-requests", UpdateTTLCount)

	GetServiceCount = metrics.NewCounter()
	metrics.Register("skydns-get-service-requests", GetServiceCount)

	RemoveServiceCount = metrics.NewCounter()
	metrics.Register("skydns-remove-service-requests", RemoveServiceCount)
}

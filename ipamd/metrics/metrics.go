package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	test *prometheus.GaugeVec
}

func New() (*Metrics, error) {
	m := &Metrics{}

	return m, nil
}

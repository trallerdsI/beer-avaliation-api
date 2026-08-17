package moderation

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var ModerationRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "moderation_requests_total",
		Help: "Total number of content moderation requests by status",
	},
	[]string{"status"},
)

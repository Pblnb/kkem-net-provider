package service

import "time"

const (
	DnsZoneType = "private"
	RecordSet   = "A"

	PollingInterval = 5 * time.Second
	PollingTimeout  = 5 * time.Minute

	pollingErrTolerance = 3

	VpcepStatusAvailable = "accepted"
	VpcepStatusFailed    = "failed"
	VpcepStatusCreating  = "creating"
)

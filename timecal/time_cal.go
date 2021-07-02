package timecal

import (
	"sync"
	"time"
)

type TimeCalculator interface {
	RecvTs(commandD string)

	DoneTs(commandD string) []time.Duration
}

type timeCalculator struct {
	mutex sync.Mutex

	receivedTime map[string]time.Time

	finishedTime map[string]time.Time
}

func NewTimeCal() TimeCalculator {
	return &timeCalculator{receivedTime: make(map[string]time.Time), finishedTime: make(map[string]time.Time)}
}

func (tc *timeCalculator) RecvTs(commandD string) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	tc.receivedTime[commandD] = time.Now()
}

func (tc *timeCalculator) DoneTs(commandD string) []time.Duration {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	tc.finishedTime[commandD] = time.Now()

	var latencySet []time.Duration

	for digest := range tc.finishedTime {
		recv, ok := tc.receivedTime[digest]
		if !ok {
			continue
		}

		latency := tc.finishedTime[commandD].Sub(recv)
		delete(tc.receivedTime, digest)
		delete(tc.finishedTime, digest)

		latencySet = append(latencySet, latency)
	}

	return latencySet
}

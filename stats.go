package main

import "time"

type StatCmdRecordRequest struct {
	latency time.Duration
}

type StatCmdRetrieve struct {
	resp chan int64
}

type StatsStore struct {
	count        int64
	totalLatency time.Duration
}

func startStatsLoop() chan<- interface{} {
	statsStore := StatsStore{
		count:        0,
		totalLatency: 0,
	}

	cmds := make(chan interface{}, 100)

	go func() {
		for cmd := range cmds {
			switch cmd.(type) {
			case StatCmdRecordRequest:
				statsStore.count++
				statsStore.totalLatency += cmd.(StatCmdRecordRequest).latency
			case StatCmdRetrieve:
				resp := cmd.(StatCmdRetrieve).resp
				resp <- statsStore.count
				resp <- statsStore.totalLatency.Microseconds()
			default:
				// TODO log
			}
		}
	}()

	return cmds
}

type StatsJson struct {
	Total   int64 `json:"total"`
	Average int64 `json:"average"`
}

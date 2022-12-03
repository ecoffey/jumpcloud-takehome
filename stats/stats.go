package stats

import (
	"log"
	"time"
)

type StatsJson struct {
	Total   int64 `json:"total"`
	Average int64 `json:"average"`
}

type StatCmdRecordRequest struct {
	Latency time.Duration
}

type StatCmdRetrieve struct {
	Resp chan int64
}

type statsStore struct {
	count        int64
	totalLatency time.Duration
}

// StartStatsLoop builds and returns a channel of empty interface, where the
// intention is to feed that channel StatCmd* types, and begins consuming from
// it in a go routine.
func StartStatsLoop() chan<- interface{} {
	s := statsStore{
		count:        0,
		totalLatency: 0,
	}

	cmds := make(chan interface{}, 100)

	go func() {
		for cmd := range cmds {
			switch cmd.(type) {
			case StatCmdRecordRequest:
				s.processRecordCmd(cmd.(StatCmdRecordRequest))
			case StatCmdRetrieve:
				s.processRetrieveCmd(cmd.(StatCmdRetrieve))
			default:
				log.Fatalln("unknown command type")
			}
		}
	}()

	return cmds
}

func (s *statsStore) processRecordCmd(cmd StatCmdRecordRequest) {
	s.count++
	s.totalLatency += cmd.Latency
}

func (s *statsStore) processRetrieveCmd(cmd StatCmdRetrieve) {
	cmd.Resp <- s.count
	cmd.Resp <- s.totalLatency.Microseconds()
}

package main

import (
	"encoding/json"
	"eoinisawesome.com/jumpcloud-takehome/hashes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type App struct {
	hashCmds chan<- interface{}
	statCmds chan<- interface{}
}

func (s *App) hashEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		start := time.Now()
		defer func() {
			end := time.Now()
			go func() {
				s.statCmds <- StatCmdRecordRequest{latency: end.Sub(start)}
			}()
		}()

		if r.ParseForm() != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		resp := make(chan int)
		s.hashCmds <- hashes.HashCmdReserveId{
			Plaintext: r.Form.Get("password"),
			Resp:      resp,
		}
		id := <-resp
		fmt.Fprintf(w, "%d", id)
	} else if r.Method == http.MethodGet {
		idStr := strings.TrimPrefix(r.URL.Path, "/hash/")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		resp := make(chan string)
		s.hashCmds <- hashes.HashCmdRetrieve{Id: id, Resp: resp}
		hash := <-resp
		fmt.Fprintf(w, "%s", hash)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *App) statsEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp := make(chan int64)
	s.statCmds <- StatCmdRetrieve{resp: resp}
	totalRequests := <-resp
	totalLatency := <-resp

	var jsonStruct = StatsJson{}
	if totalRequests > 0 {
		jsonStruct = StatsJson{
			Total:   totalRequests,
			Average: totalLatency / totalRequests,
		}
	}

	jsonBytes, err := json.Marshal(jsonStruct)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, string(jsonBytes))
}

func (s *App) shutdownEndpoint(w http.ResponseWriter, r *http.Request) {
	s.hashCmds <- hashes.HashCmdGracefulShutdown{}
}

func router(shutdown chan int, hashDelay time.Duration) http.Handler {
	server := App{
		hashCmds: hashes.StartHashLoop(shutdown, hashDelay),
		statCmds: startStatsLoop(),
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/hash", server.hashEndpoint)
	mux.HandleFunc("/hash/", server.hashEndpoint)
	mux.HandleFunc("/stats", server.statsEndpoint)
	mux.HandleFunc("/shutdown", server.shutdownEndpoint)

	return mux
}

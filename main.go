package main

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type HashCmdReserveId struct {
	plaintext string
	hashDelay time.Duration
	resp      chan int
}

type HashCmdStore struct {
	id   int
	hash string
}

type HashCmdRetrieve struct {
	id   int
	resp chan string
}

type HashCmdGracefulShutdown struct{}

type RecordRequestCmd struct {
	latency time.Duration
}

type RetrieveStatsCmd struct {
	resp chan int64
}

type HashesStore struct {
	id       int
	idToHash map[int]string
}

type StatsStore struct {
	count        int64
	totalLatency time.Duration
}

type Server struct {
	hashCmds  chan<- interface{}
	statsCmds chan<- interface{}
	hashDelay time.Duration
}

func hashEncode(plaintext string) string {
	hasher := sha512.New()
	hasher.Write([]byte(plaintext))
	return base64.StdEncoding.EncodeToString(hasher.Sum(nil))
}

func startHashLoop(shutdown chan int) chan<- interface{} {
	hashStore := HashesStore{
		id:       1,
		idToHash: make(map[int]string),
	}

	acceptingNewHashes := true
	inFlight := 0

	cmds := make(chan interface{}, 100)

	go func() {
		for cmd := range cmds {
			switch cmd.(type) {
			case HashCmdReserveId:
				if acceptingNewHashes {
					reserveIdCmd := cmd.(HashCmdReserveId)
					id := hashStore.id
					reserveIdCmd.resp <- id
					hashStore.id += 1
					inFlight++
					if reserveIdCmd.hashDelay > 0 {
						go func() {
							<-time.Tick(reserveIdCmd.hashDelay)
							hash := hashEncode(reserveIdCmd.plaintext)
							cmds <- HashCmdStore{id: id, hash: hash}
						}()
					} else {
						hash := hashEncode(reserveIdCmd.plaintext)
						hashStore.idToHash[id] = hash
						inFlight--
						if !acceptingNewHashes && inFlight == 0 {
							// signal on shutdown channel
							shutdown <- 1
						}
					}
				} else {
					cmd.(HashCmdReserveId).resp <- -1
				}
			case HashCmdStore:
				addHashCmd := cmd.(HashCmdStore)
				hashStore.idToHash[addHashCmd.id] = addHashCmd.hash
				inFlight--
				if !acceptingNewHashes && inFlight == 0 {
					// signal on shutdown channel
					shutdown <- 1
				}
			case HashCmdRetrieve:
				retrieveHashCmd := cmd.(HashCmdRetrieve)
				retrieveHashCmd.resp <- hashStore.idToHash[retrieveHashCmd.id]
			case HashCmdGracefulShutdown:
				acceptingNewHashes = false
				if inFlight == 0 {
					shutdown <- 1
				}
			default:
				log.Fatalln("unknown command type")
			}
		}
	}()

	return cmds
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
			case RecordRequestCmd:
				statsStore.count++
				statsStore.totalLatency += cmd.(RecordRequestCmd).latency
			case RetrieveStatsCmd:
				resp := cmd.(RetrieveStatsCmd).resp
				resp <- statsStore.count
				resp <- statsStore.totalLatency.Microseconds()
			default:
				// TODO log
			}
		}
	}()

	return cmds
}

func (s *Server) hashEndpoint(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.Method == http.MethodPost {
		if r.ParseForm() != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		resp := make(chan int)
		s.hashCmds <- HashCmdReserveId{
			plaintext: r.Form.Get("password"),
			hashDelay: s.hashDelay,
			resp:      resp,
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
		s.hashCmds <- HashCmdRetrieve{id: id, resp: resp}
		hash := <-resp
		fmt.Fprintf(w, "%s", hash)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
	end := time.Now()
	go func() {
		s.statsCmds <- RecordRequestCmd{latency: end.Sub(start)}
	}()
}

type StatsJson struct {
	Total   int64 `json:"total"`
	Average int64 `json:"average"`
}

func (s *Server) statsEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp := make(chan int64)
	s.statsCmds <- RetrieveStatsCmd{resp: resp}
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

func (s *Server) shutdownEndpoint(w http.ResponseWriter, r *http.Request) {
	s.hashCmds <- HashCmdGracefulShutdown{}
}

func router(shutdown chan int, hashDelay time.Duration) http.Handler {
	server := Server{
		hashCmds:  startHashLoop(shutdown),
		statsCmds: startStatsLoop(),
		hashDelay: hashDelay,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/hash", server.hashEndpoint)
	mux.HandleFunc("/hash/", server.hashEndpoint)
	mux.HandleFunc("/stats", server.statsEndpoint)
	mux.HandleFunc("/shutdown", server.shutdownEndpoint)

	return mux
}

func main() {
	println("starting")
	shutdown := make(chan int)
	httpServer := http.Server{Addr: ":3333", Handler: router(shutdown, 5*time.Second)}

	go func() {
		println("waiting for shutdown signal...")
		<-shutdown
		println("got signal calling close!")
		httpServer.Close()
	}()

	err := httpServer.ListenAndServe()

	if errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("server closed\n")
	} else if err != nil {
		fmt.Printf("error starting server: %s\n", err)
		os.Exit(1)
	}

}

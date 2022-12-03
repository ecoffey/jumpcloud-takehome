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

type ReserveIdCmd struct {
	plaintext string
	hashDelay time.Duration
	resp      chan int
}

type StoreHashCmd struct {
	id   int
	hash string
}

type RetrieveHashCmd struct {
	id   int
	resp chan string
}

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

func startHashLoop() chan<- interface{} {
	hashStore := HashesStore{
		id:       1,
		idToHash: make(map[int]string),
	}

	cmds := make(chan interface{})

	go func() {
		for cmd := range cmds {
			switch cmd.(type) {
			case ReserveIdCmd:
				reservedIdCmd := cmd.(ReserveIdCmd)
				id := hashStore.id
				reservedIdCmd.resp <- id
				hashStore.id += 1
				if reservedIdCmd.hashDelay > 0 {
					go func() {
						<-time.Tick(reservedIdCmd.hashDelay)
						hash := hashEncode(reservedIdCmd.plaintext)
						cmds <- StoreHashCmd{id: id, hash: hash}
					}()
				} else {
					hash := hashEncode(reservedIdCmd.plaintext)
					hashStore.idToHash[id] = hash
				}
			case StoreHashCmd:
				addHashCmd := cmd.(StoreHashCmd)
				hashStore.idToHash[addHashCmd.id] = addHashCmd.hash
			case RetrieveHashCmd:
				retrieveHashCmd := cmd.(RetrieveHashCmd)
				retrieveHashCmd.resp <- hashStore.idToHash[retrieveHashCmd.id]

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

	cmds := make(chan interface{})

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
		s.hashCmds <- ReserveIdCmd{
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
		s.hashCmds <- RetrieveHashCmd{id: id, resp: resp}
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

	var jsonStruct StatsJson
	if totalRequests > 0 {
		jsonStruct = StatsJson{
			Total:   totalRequests,
			Average: totalLatency / totalRequests,
		}
	} else {
		jsonStruct = StatsJson{
			Total:   0,
			Average: 0,
		}
	}

	jsonBytes, err := json.Marshal(jsonStruct)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, string(jsonBytes))
}

func router(hashDelay time.Duration) http.Handler {
	server := Server{
		hashCmds:  startHashLoop(),
		statsCmds: startStatsLoop(),
		hashDelay: hashDelay,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/hash", server.hashEndpoint)
	mux.HandleFunc("/hash/", server.hashEndpoint)
	mux.HandleFunc("/stats", server.statsEndpoint)

	return mux
}

func main() {
	err := http.ListenAndServe(":3333", router(5*time.Second))

	if errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("server closed\n")
	} else if err != nil {
		fmt.Printf("error starting server: %s\n", err)
		os.Exit(1)
	}
}

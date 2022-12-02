package main

import (
	"crypto/sha512"
	"encoding/base64"
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

type HashesStore struct {
	id       int
	idToHash map[int]string
}
type Server struct {
	cmds      chan<- interface{}
	hashDelay time.Duration
}

func hashEncode(plaintext string) string {
	hasher := sha512.New()
	hasher.Write([]byte(plaintext))
	return base64.StdEncoding.EncodeToString(hasher.Sum(nil))
}

func startHashesStoreManager() chan<- interface{} {
	store := HashesStore{id: 1, idToHash: make(map[int]string)}

	cmds := make(chan interface{})

	go func() {
		for cmd := range cmds {
			switch cmd.(type) {
			case ReserveIdCmd:
				reservedIdCmd := cmd.(ReserveIdCmd)
				id := store.id
				reservedIdCmd.resp <- id
				store.id += 1
				if reservedIdCmd.hashDelay > 0 {
					go func() {
						<-time.Tick(reservedIdCmd.hashDelay)
						hash := hashEncode(reservedIdCmd.plaintext)
						cmds <- StoreHashCmd{id: id, hash: hash}
					}()
				} else {
					hash := hashEncode(reservedIdCmd.plaintext)
					store.idToHash[id] = hash
				}
			case StoreHashCmd:
				addHashCmd := cmd.(StoreHashCmd)
				store.idToHash[addHashCmd.id] = addHashCmd.hash
			case RetrieveHashCmd:
				retrieveHashCmd := cmd.(RetrieveHashCmd)
				retrieveHashCmd.resp <- store.idToHash[retrieveHashCmd.id]
			default:
				log.Fatalln("unknown command type")
			}
		}
	}()

	return cmds
}

func (s *Server) hashEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if r.ParseForm() != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		resp := make(chan int)
		s.cmds <- ReserveIdCmd{
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
		s.cmds <- RetrieveHashCmd{id: id, resp: resp}
		hash := <-resp
		fmt.Fprintf(w, "%s", hash)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func router(hashDelay time.Duration) http.Handler {
	server := Server{
		cmds:      startHashesStoreManager(),
		hashDelay: hashDelay,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/hash", server.hashEndpoint)
	mux.HandleFunc("/hash/", server.hashEndpoint)

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

package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
)

type CommandType int

const (
	AddHash = iota
)

type CmdRequest struct {
	commandType CommandType
	replyChan   chan int
}

type HashesStore struct {
	id int
}
type Server struct {
	cmds chan<- CmdRequest
}

func startHashesStoreManager() chan<- CmdRequest {
	store := HashesStore{id: 1}

	cmds := make(chan CmdRequest)

	go func() {
		for cmd := range cmds {
			switch cmd.commandType {
			case AddHash:
				cmd.replyChan <- store.id
				store.id += 1
			default:
				log.Fatalln("unknown command type", cmd.commandType)
			}
		}
	}()

	return cmds
}

func (s *Server) addHash(w http.ResponseWriter, _ *http.Request) {
	replyChan := make(chan int)
	s.cmds <- CmdRequest{commandType: AddHash, replyChan: replyChan}

	id := <-replyChan
	fmt.Fprintf(w, "%d\n", id)
}

func main() {
	cmds := startHashesStoreManager()
	server := Server{cmds: cmds}

	mux := http.NewServeMux()

	mux.HandleFunc("/", server.addHash)

	err := http.ListenAndServe(":3333", mux)

	if errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("server closed\n")
	} else if err != nil {
		fmt.Printf("error starting server: %s\n", err)
		os.Exit(1)
	}
}

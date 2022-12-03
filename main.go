package main

import (
	"errors"
	"net/http"
	"os"
	"time"
)

func main() {
	println("starting")
	shutdown := make(chan int)
	httpServer := http.Server{
		Addr:    ":3333",
		Handler: router(shutdown, 5*time.Second),
	}

	go func() {
		println("waiting for shutdown signal...")
		<-shutdown
		println("got signal calling close!")
		httpServer.Close()
	}()

	err := httpServer.ListenAndServe()

	if errors.Is(err, http.ErrServerClosed) {
		println("server closed\n")
	} else if err != nil {
		println("error starting server: %s\n", err)
		os.Exit(1)
	}

}

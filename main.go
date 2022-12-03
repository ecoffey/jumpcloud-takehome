package main

import (
	"eoinisawesome.com/jumpcloud-takehome/app"
	"errors"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	log.Println("starting")
	shutdown := make(chan int)
	httpServer := http.Server{
		Addr:    ":3333",
		Handler: app.Router(shutdown, 5*time.Second),
	}

	go func() {
		log.Println("waiting for shutdown signal...")
		<-shutdown
		log.Println("got signal calling close!")
		httpServer.Close()
	}()

	err := httpServer.ListenAndServe()

	if errors.Is(err, http.ErrServerClosed) {
		log.Println("server closed\n")
	} else if err != nil {
		log.Fatalln("error starting server: %s\n", err)
		os.Exit(1)
	}

}

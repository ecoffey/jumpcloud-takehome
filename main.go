package main

import (
	"eoinisawesome.com/jumpcloud-takehome/app"
	"errors"
	"log"
	"net/http"
	"time"
)

func main() {
	log.Println("starting")
	shutdown := make(chan int)
	httpServer := http.Server{
		Addr:    ":8080",
		Handler: app.AppRouter(shutdown, 5*time.Second),
	}

	go func() {
		log.Println("waiting for shutdown signal...")
		<-shutdown
		log.Println("got signal calling close!")
		err := httpServer.Close()
		if err != nil {
			log.Fatalln("unable to close server", err)
			return
		}
	}()

	err := httpServer.ListenAndServe()

	if errors.Is(err, http.ErrServerClosed) {
		log.Println("server closed")
	} else if err != nil {
		log.Fatalln("error starting server", err)
	}

}

package main

import (
	"log"

	"github.com/kahlys/proxy"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {

	s := &proxy.Server{
		Addr:   ":5279",
		Target: "server.growatt.com:5279",
		ModifyRequest: func(req *[]byte) {
			log.Println("request: " + string(*req))
		},
		ModifyResponse: func(req *[]byte) {
			log.Println("response: " + string(*req))
		},
	}

	return s.ListenAndServe()
}

package main

import (
	"flag"
	"log"
	"mafia-core/client"
	"mafia-core/server"
)

var (
	mode = flag.String("mode", "server", "Server or Client mode")
	port = flag.Int("port", 8080, "Server port")
)

func main() {
	flag.Parse()
	log.Printf("Starting %s", *mode)
	if *mode == "server" {
		server.Run(*port)
	} else {
		client.Run()
	}
}

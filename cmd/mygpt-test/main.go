package main

import "github.com/washsky/mygpt-test/internal/server"

var version = "dev"

func main() {
	server.Start(version)
}

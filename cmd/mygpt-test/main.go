package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/washsky/mygpt-test/internal/server"
)

var version = "dev"

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dataDir := flag.String("data-dir", "", "persistent data directory (default: beside the executable)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("mygpt-test %s (%s)\n", version, runtime.Version())
		return
	}

	if err := server.Start(*addr, version, *dataDir); err != nil {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}

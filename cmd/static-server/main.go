package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := flag.Int("port", 9000, "Port to listen on")
	dir := flag.String("dir", ".", "Directory of static files to serve")
	flag.Parse()

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	fileServer := http.FileServer(http.Dir(*dir))
	log.Printf("Cuan static file server listening on %s (dir: %s)\n", addr, *dir)
	if err := http.ListenAndServe(addr, fileServer); err != nil {
		fmt.Fprintf(os.Stderr, "Static server error: %v\n", err)
		os.Exit(1)
	}
}

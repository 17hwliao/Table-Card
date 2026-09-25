package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/17hwliao/table-card-independent/internal/server"
)

func main() {
	address := flag.String("listen", ":8080", "HTTP 监听地址")
	flag.Parse()

	httpServer := &http.Server{
		Addr:              *address,
		Handler:           server.New().Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stopped
		_ = httpServer.Close()
	}()

	log.Printf("牌桌大厅已启动：http://localhost%s", *address)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

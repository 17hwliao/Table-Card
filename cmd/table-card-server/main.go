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
	address := flag.String("listen", ":1781", "牌桌服务端监听地址")
	flag.Parse()
	service := server.New()
	defer service.Close()

	httpServer := &http.Server{
		Addr:              *address,
		Handler:           service.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stopped
		_ = httpServer.Close()
	}()

	log.Printf("牌桌终端服务已启动，监听 %s", *address)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

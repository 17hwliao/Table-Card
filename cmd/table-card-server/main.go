package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/17hwliao/table-card-independent/internal/memory"
	"github.com/17hwliao/table-card-independent/internal/server"
)

func main() {
	memory.Server()
	address := flag.String("listen", ":1781", "牌桌服务端监听地址")
	flag.Parse()
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatal(err)
	}
	service := server.New()
	service.SetListenAddress(listener.Addr())
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

	log.Printf("牌桌终端服务已启动，监听 %s", listener.Addr())
	info := server.ConnectionInfoFor(listener.Addr())
	fmt.Println("\n牌桌服务已启动 · 客户端连接地址")
	if info.Local != "" {
		fmt.Printf("仅本机使用：%s\n", info.Local)
	}
	if len(info.Addresses) > 0 {
		fmt.Println("同一局域网的玩家，在客户端输入以下地址：")
		for _, entry := range info.Addresses {
			fmt.Printf("  %s  （%s）\n", entry.Address, entry.Interface)
		}
		fmt.Println("多个地址时，选择与玩家同一网络的 Wi-Fi / 以太网地址。")
	} else {
		fmt.Println("未发现可分享的网卡地址；检查网络连接及服务监听范围。")
	}
	fmt.Println("客户端进入大厅后，再用房间号加入。连接需网络与防火墙允许。\n服务保持运行，Ctrl+C 停止。")
	if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

package main

import (
	"flag"
	"log"

	tea "charm.land/bubbletea/v2"
	"github.com/17hwliao/table-card-independent/internal/terminal/app"
)

func main() {
	server := flag.String("server", "localhost:1781", "牌桌服务端地址")
	name := flag.String("name", "玩家 1", "默认玩家名称")
	soloSnake := flag.Bool("snake", false, "直接打开离线单机贪吃蛇")
	flag.Parse()

	var model *app.Model
	if *soloSnake {
		model = app.NewSnake(*name)
	} else {
		model = app.New(*server, *name)
	}
	defer model.Close()
	program := tea.NewProgram(model)
	if _, err := program.Run(); err != nil {
		log.Fatal(err)
	}
}

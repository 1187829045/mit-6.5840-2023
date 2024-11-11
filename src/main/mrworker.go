package main

// 启动一个工作进程，该进程在 ../mr/worker.go 中实现。
// 通常会有多个工作进程与一个协调器进程进行通信。
// 使用命令：go run mrworker.go wc.so
// 请不要修改这个文件。
//

import "6.5840/mr"
import "plugin"
import "os"
import "fmt"
import "log"

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "用法: mrworker xxx.so\n")
		os.Exit(1)
	}
	//fmt.Println("工作进行中")
	mapf, reducef := loadPlugin(os.Args[1])

	mr.Worker(mapf, reducef)
}

// 从插件文件中加载应用的 Map 和 Reduce 函数
// 例如：../mrapps/wc.so

func loadPlugin(filename string) (func(string, string) []mr.KeyValue, func(string, []string) string) {
	p, err := plugin.Open(filename)
	if err != nil {
		log.Fatalf("无法加载插件 %v", filename)
	}
	xmapf, err := p.Lookup("Map")
	if err != nil {
		log.Fatalf("无法找到 %v 中的 Map", filename)
	}
	mapf := xmapf.(func(string, string) []mr.KeyValue)
	xreducef, err := p.Lookup("Reduce")
	if err != nil {
		log.Fatalf("无法找到 %v 中的 Reduce", filename)
	}
	reducef := xreducef.(func(string, []string) string)

	return mapf, reducef
}

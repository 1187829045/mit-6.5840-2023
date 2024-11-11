package main

//
// 启动协调者进程，协调者的实现位于
// ../mr/coordinator.go
//
// 运行命令： go run mrcoordinator.go pg*.txt
//
// 请不要修改此文件。
//

import "6.5840/mr"
import "time"
import "os"
import "fmt"

func main() {
	// 如果命令行参数小于 2 个，提示使用方法并退出
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "用法:mrcoordinator inputfiles...\n")
		os.Exit(1)
	}

	// 创建一个协调者，传入输入文件和 worker 数量
	m := mr.MakeCoordinator(os.Args[1:], 10)
	// 当协调者未完成时，不断休眠
	for m.Done() == false {
		//fmt.Println("sleeping")
		time.Sleep(time.Second)
	}

	// 最后再等一秒钟，以确保所有操作完成
	time.Sleep(time.Second)
}

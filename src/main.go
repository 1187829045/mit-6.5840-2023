package main

import (
	"fmt"
)

type KeyValue struct {
	Key   string
	Value string
}

func main() {
	for i := 0; i < 10; i++ {
		fileName := fmt.Sprintf("mr-%v", i) //中间文件名称
		fmt.Println(fileName)
	}
}

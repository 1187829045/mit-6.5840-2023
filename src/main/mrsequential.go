package main

//
// 简单的顺序 MapReduce 实现.
//
// go run mrsequential.go wc.so pg*.txt
//

import "fmt"
import "6.5840/mr"
import "plugin"
import "os"
import "log"
import "io/ioutil"
import "sort"

// 按键排序的结构体

type ByKey []mr.KeyValue

// 为排序提供接口方法

func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

func main() {
	// 如果命令行参数不足，打印用法并退出
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: mrsequential xxx.so inputfiles...\n")
		os.Exit(1)
	}

	// 加载插件中的 Map 和 Reduce 函数
	mapf, reducef := loadPlugin(os.Args[1])
	//
	// 读取每个输入文件，
	// 传递给 Map 函数，
	// 累积中间 Map 输出结果。
	//
	intermediate := []mr.KeyValue{} // 用于存储中间结果
	for _, filename := range os.Args[2:] {
		// 打开文件
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("无法打开文件 %v", filename)
		}
		// 读取文件内容
		content, err := ioutil.ReadAll(file)
		if err != nil {
			log.Fatalf("无法读取文件 %v", filename)
		}
		file.Close() // 关闭文件
		// 调用 Map 函数处理文件内容，返回键值对
		kva := mapf(filename, string(content))
		// 将返回的键值对追加到 intermediate 列表中
		intermediate = append(intermediate, kva...)
	}

	//
	// 与真实的 MapReduce 大不相同的是，
	// 所有的中间数据都在一个地方，即 intermediate[]，
	// 而不是被分成多个桶。
	//

	// 按键排序中间结果
	sort.Sort(ByKey(intermediate))

	// 创建输出文件 mr-out-0
	oname := "mr-out-0"
	ofile, _ := os.Create(oname)

	//
	// 对 intermediate[] 中每个不同的键调用 Reduce 函数，
	// 并将结果写入 mr-out-0 文件。
	//
	i := 0
	for i < len(intermediate) {
		// 寻找相同键的连续段
		j := i + 1
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		// 提取相同键的所有值
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, intermediate[k].Value)
		}
		// 调用 Reduce 函数处理这些值
		output := reducef(intermediate[i].Key, values)

		// 将 Reduce 输出结果按正确格式写入文件
		fmt.Fprintf(ofile, "%v %v\n", intermediate[i].Key, output)

		// 更新 i 为下一组键的起始位置
		i = j
	}

	// 关闭输出文件
	ofile.Close()
}

// 从插件文件加载应用的 Map 和 Reduce 函数
// 例如：../mrapps/wc.so
func loadPlugin(filename string) (func(string, string) []mr.KeyValue, func(string, []string) string) {
	// 打开插件文件
	p, err := plugin.Open(filename)
	if err != nil {
		log.Fatalf("无法加载插件 %v", filename)
	}
	// 查找 Map 函数
	xmapf, err := p.Lookup("Map")
	if err != nil {
		log.Fatalf("无法在插件中找到 Map 函数 %v", filename)
	}
	// 将 Map 函数转换为正确的类型
	mapf := xmapf.(func(string, string) []mr.KeyValue)
	// 查找 Reduce 函数
	xreducef, err := p.Lookup("Reduce")
	if err != nil {
		log.Fatalf("无法在插件中找到 Reduce 函数 %v", filename)
	}
	// 将 Reduce 函数转换为正确的类型
	reducef := xreducef.(func(string, []string) string)

	// 返回 Map 和 Reduce 函数
	return mapf, reducef
}

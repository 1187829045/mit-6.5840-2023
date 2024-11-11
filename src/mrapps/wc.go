package main

//
// 一个用于 MapReduce 的单词计数应用 "插件"。
// go build -buildmode=plugin wc.go
//

import "6.5840/mr"
import "unicode"
import "strings"
import "strconv"

// Map 函数每次处理一个输入文件。第一个参数是输入文件的名称，第二个参数是文件的完整内容。
// 你应该忽略输入文件名，只关注内容参数。返回值是一个键值对的切片。

func Map(filename string, contents string) []mr.KeyValue {
	// 用于检测单词分隔符的函数（非字母字符即为分隔符）
	ff := func(r rune) bool { return !unicode.IsLetter(r) }

	// 使用分隔符将文件内容拆分为单词数组
	words := strings.FieldsFunc(contents, ff)

	kva := []mr.KeyValue{} // 用于存储 Map 结果的切片
	for _, w := range words {
		// 创建键值对，其中键为单词，值为 "1" 表示出现了一次
		kv := mr.KeyValue{w, "1"}
		kva = append(kva, kv)
	}
	return kva // 返回包含所有单词键值对的切片
}

// Reduce 函数每次处理 Map 阶段生成的一个键及其对应的值列表。
// 参数 key 是当前处理的键（单词），values 是所有 Map 任务为该键生成的值（表示单词出现次数）的列表。
// 该函数返回该键的最终值，即该单词在所有文件中的出现次数。

func Reduce(key string, values []string) string {
	// 返回该单词出现的次数，值为 values 列表的长度
	return strconv.Itoa(len(values))
}

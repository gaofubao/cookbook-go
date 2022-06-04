package main

import (
	"fmt"
	"sort"
)

func main() {
	m := map[int]string{1: "a", 2: "b", 3: "c"}
	fmt.Println("first range:")
	for i, v := range m {
		fmt.Printf("m[%v]=%v\n", i, v)
	}
	fmt.Println("second range:")
	for i, v := range m {
		fmt.Printf("m[%v]=%v\n", i, v)
	}

	// 实现有序遍历
	var sl []int
	// 把 key 单独取出放到切片
	for k := range m {
		sl = append(sl, k)
	}
	// 排序切片
	sort.Ints(sl)
	// 以切片中的 key 顺序遍历 map 就是有序的了
	for _, k := range sl {
		fmt.Println(k, m[k])
	}
}

package main

import (
	"fmt"
	"time"
)

// 非线程安全
func main() {
	s := make(map[int]int)

	for i := 0; i < 100; i++ {
		go func(i int) {
			s[i] = i
		}(i)
	}

	for i := 0; i < 100; i++ {
		go func(i int) {
			fmt.Printf("map第%d个元素值是%d\n", i, s[i])
		}(i)
	}
	time.Sleep(1 * time.Second)
}

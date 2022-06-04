package main

import (
	"fmt"
	"runtime"
	"time"
)

// channel 发送不接收
func block() {
	ch := make(chan int)
	for i := 0; i < 10; i++ {
		go func() {
			ch <- 1
		}()
	}
}

func main() {
	fmt.Println("before goroutines: ", runtime.NumGoroutine())
	block()
	time.Sleep(time.Second * 1)
	fmt.Println("after goroutines: ", runtime.NumGoroutine())
}

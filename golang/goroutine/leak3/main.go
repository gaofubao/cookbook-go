package main

import (
	"fmt"
	"runtime"
	"time"
)

// channel 接收不发送
func block() {
	ch := make(chan int)
	for i := 0; i < 10; i++ {
		go func() {
			<-ch
		}()
	}
}

func main() {
	fmt.Println("before goroutines: ", runtime.NumGoroutine())
	block()
	time.Sleep(time.Second * 1)
	fmt.Println("after goroutines: ", runtime.NumGoroutine())
}

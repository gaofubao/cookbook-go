package main

import (
	"fmt"
	"runtime"
	"time"
)

// nil channel
func block() {
	var ch chan int
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

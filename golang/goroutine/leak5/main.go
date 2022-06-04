package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// 互斥锁忘记解锁
func block() {
	var mutex sync.Mutex
	for i := 0; i < 10; i++ {
		go func() {
			mutex.Lock()
		}()
	}
}

func main() {
	fmt.Println("before goroutines: ", runtime.NumGoroutine())
	block()
	time.Sleep(time.Second * 1)
	fmt.Println("after goroutines: ", runtime.NumGoroutine())
}

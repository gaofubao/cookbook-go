package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// sync.WaitGroup 使用不当
func block() {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		go func() {
			wg.Add(2)
			wg.Done()
			wg.Wait()
		}()
	}
}

func main() {
	fmt.Println("before goroutines: ", runtime.NumGoroutine())
	block()
	time.Sleep(time.Second * 1)
	fmt.Println("after goroutines: ", runtime.NumGoroutine())
}

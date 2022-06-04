package main

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"
)

var wg sync.WaitGroup

// http request body 未关闭
func requestWithNoClose() {
	resp, err := http.Get("https://www.baidu.com")
	if err != nil {
		fmt.Printf("error occurred while fetching page, error code: %d, error: %s", resp.StatusCode, err.Error())
	}
	// defer resp.Body.Close()
}

func block() {
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			requestWithNoClose()
		}()
	}
}

func main() {
	fmt.Println("before goroutines: ", runtime.NumGoroutine())
	block()
	time.Sleep(time.Second * 1)
	fmt.Println("after goroutines: ", runtime.NumGoroutine())
	wg.Wait()
}

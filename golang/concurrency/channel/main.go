package main

import (
	"fmt"
	"time"
)

// channel
func main() {
	ch := make(chan struct{}, 1)
	for i := 0; i < 10; i++ {
		go func() {
			ch <- struct{}{}
			time.Sleep(1 * time.Second)
			fmt.Println("通过channel访问临界区")
			<-ch
		}()
	}

	select {}
}

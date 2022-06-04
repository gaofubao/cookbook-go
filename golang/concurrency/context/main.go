package main

import (
	"context"
	"fmt"
	"time"
)

// context
func main() {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer func() {
			fmt.Println("goroutine exit")
		}()

		for {
			select {
			case <-ctx.Done():
				fmt.Println("receive cancel signal!")
				return
			default:
				fmt.Println("default")
				time.Sleep(time.Second)
			}
		}
	}()

	time.Sleep(time.Second)
	cancel()
	time.Sleep(2 * time.Second)
}

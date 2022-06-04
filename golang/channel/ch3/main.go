package main

import (
	"fmt"
	"time"
)

func main() {
	ch := make(chan int, 3)
	ch <- 1
	ch <- 2
	ch <- 3
	ch <- 4
	go loop(ch)
	time.Sleep(1 * time.Millisecond)
}

func loop(ch chan int) {
	for {
		select {
		case i := <-ch:
			fmt.Println("this value of unbuffer channel", i)
		}
	}
}

package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	var m sync.Map

	for i := 0; i < 100; i++ {
		go func(i int) {
			m.Store(i, i)
		}(i)
	}

	for i := 0; i < 100; i++ {
		go func(i int) {
			v, ok := m.Load(i)
			fmt.Printf("Load: %v, %v\n", v, ok)
		}(i)
	}
	time.Sleep(1 * time.Second)
}

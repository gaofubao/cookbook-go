package main

import (
	"fmt"
	"sync"
)

// once
func main() {
	once := &sync.Once{}

	for i := 0; i < 10; i++ {
		once.Do(func() {
			fmt.Println("only once")
		})
	}
}

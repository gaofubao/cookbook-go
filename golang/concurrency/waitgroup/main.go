package main

import (
	"fmt"
	"sync"
)

// waitgroup
func main() {
	list := []string{"A", "B", "C", "D"}
	wg := &sync.WaitGroup{}
	wg.Add(len(list))

	for _, char := range list {
		go func(char string) {
			defer wg.Done()
			fmt.Println(char)
		}(char)
	}
	wg.Wait()
}

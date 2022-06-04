package main

import (
	"fmt"
	"sync"
)

func main() {
	a := make([]int, 0)
	var wg sync.WaitGroup

	for i := 0; i < 10000; i++ {
		wg.Add(1)
		go func(i int) {
			a = append(a, i)
			wg.Done()
		}(i)
	}

	wg.Wait()
	fmt.Println(len(a))
}

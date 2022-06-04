package main

import "fmt"

// 浅拷贝
func main() {
	slice1 := []int{1, 2, 3, 4, 5}
	fmt.Printf("slice1: %v, %p\n", slice1, slice1)

	slice2 := slice1
	fmt.Printf("slice2: %v, %p\n", slice2, slice2)
}

package main

import "fmt"

// 深拷贝
func main() {
	slice1 := []int{1, 2, 3, 4, 5}
	fmt.Printf("slice1: %v, %p\n", slice1, slice1)

	slice2 := make([]int, 5, 5)
	copy(slice2, slice1)
	fmt.Printf("slice2: %v, %p\n", slice2, slice2)

	slice3 := make([]int, 0, 5)
	for _, v := range slice1 {
		slice3 = append(slice3, v)
	}
	fmt.Printf("slice3: %v, %p\n", slice3, slice3)
}

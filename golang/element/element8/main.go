package main

import "fmt"

// defer：延迟函数执行，先进后出
func main() {
	defer fmt.Println("defer1")
	defer fmt.Println("defer2")
	defer fmt.Println("defer3")
	defer fmt.Println("defer4")
	fmt.Println("end")
}

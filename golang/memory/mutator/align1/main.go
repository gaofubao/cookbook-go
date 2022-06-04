package main

import (
	"fmt"
	"unsafe"
)

// 查看对齐系数
func main() {
	fmt.Printf("bool alignof is %d\n", unsafe.Alignof(bool(true)))    // 1
	fmt.Printf("string alignof is %d\n", unsafe.Alignof(string("a"))) // 8
	fmt.Printf("int8 alignof is %d\n", unsafe.Alignof(int8(0)))       // 1
	fmt.Printf("int16 alignof is %d\n", unsafe.Alignof(int16(0)))     // 2
	fmt.Printf("int32 alignof is %d\n", unsafe.Alignof(int32(0)))     // 4
	fmt.Printf("int64 alignof is %d\n", unsafe.Alignof(int64(0)))     // 8
	fmt.Printf("int alignof is %d\n", unsafe.Alignof(int(0)))         // 8
	fmt.Printf("float32 alignof is %d\n", unsafe.Alignof(float32(0))) // 4
	fmt.Printf("float64 alignof is %d\n", unsafe.Alignof(float64(0))) // 8
}

package main

// 指针逃逸
func escape() *int {
	var a int = 1
	return &a
}

func main() {
	escape()
}

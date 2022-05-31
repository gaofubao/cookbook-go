package main

// 闭包引用对象
func escape5() func() int {
	var i int = 1

	return func() int {
		i++
		return i
	}
}

func main() {
	escape5()
}

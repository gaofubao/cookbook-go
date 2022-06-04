package main

// 栈空间不足
func escape() {
	s := make([]int, 0, 10000)

	for index := range s {
		s[index] = index
	}
}

func main() {
	escape()
}

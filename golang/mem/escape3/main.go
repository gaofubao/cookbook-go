package main

// 变量大小不确定
func escape() {
	number := 10
	s := make([]int, number) // 编译期间无法确定slice的长度

	for i := 0; i < len(s); i++ {
		s[i] = i
	}
}

func main() {
	escape()
}

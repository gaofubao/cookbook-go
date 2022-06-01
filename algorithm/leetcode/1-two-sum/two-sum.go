package main

// 枚举法
// 时间复杂度 O(n^2)
// 空间复杂度 O(1)
func twoSum1(nums []int, target int) []int {
	length := len(nums)
	for i := 0; i < length; i++ {
		for j := i + 1; j < length; j++ {
			if nums[i]+nums[j] == target {
				return []int{i, j}
			}
		}
	}

	return nil
}

// 哈希表法
// 时间复杂度 O(n)
// 空间复杂度 O(n)
func twoSum2(nums []int, target int) []int {
	hashTable := make(map[int]int)

	for i, num := range nums {
		if index, ok := hashTable[target-num]; ok {
			return []int{index, i}
		}

		hashTable[num] = i
	}

	return nil
}

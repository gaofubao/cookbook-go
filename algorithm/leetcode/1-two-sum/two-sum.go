package main

import "sort"

/* 两数之和 */

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

/* 两数之和变种 */

func twoSum3(nums []int, target int) []int {
	// 先排序
	sort.Ints(nums)

	left, right := 0, len(nums)-1
	for left < right {
		sum := nums[left] + nums[right]

		if sum < target {
			left++
		} else if sum > target {
			right--
		} else {
			return []int{nums[left], nums[right]}
		}
	}

	return nil
}

func twoSum4(nums []int, target int) [][]int {
	var res [][]int
	sort.Ints(nums)

	left, right := 0, len(nums)-1
	for left < right {
		leftVal, rightVal := nums[left], nums[right]
		sum := leftVal + rightVal

		if sum < target {
			for left < right && nums[left] == leftVal {
				left++
			}
		} else if sum > target {
			for left < right && nums[right] == rightVal {
				right--
			}
		} else {
			res = append(res, []int{leftVal, rightVal})
			for left < right && nums[left] == leftVal {
				left++
			}
			for left < right && nums[right] == rightVal {
				right--
			}
		}
	}

	return res
}

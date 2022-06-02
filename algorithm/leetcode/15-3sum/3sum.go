package main

import "sort"

/* 三数之和 */

func threeSum(nums []int) [][]int {
	var res [][]int
	length := len(nums)

	sort.Ints(nums)

	// 枚举 a
	for first := 0; first < length; first++ {
		// 需要和上一次枚举的数不相同
		if first > 0 && nums[first] == nums[first-1] {
			continue
		}
		// c 对应的指针初始指向数组的最右端
		third := length - 1
		target := -1 * nums[first]
		// 枚举 b
		for second := first + 1; second < length; second++ {
			// 需要和上一次枚举的数不相同
			if second > first+1 && nums[second] == nums[second-1] {
				continue
			}
			// 需要保证 b 的指针在 c 的指针的左侧
			for second < third && nums[second]+nums[third] > target {
				third--
			}
			// 如果指针重合，随着 b 后续的增加
			// 就不会有满足 a+b+c=0 并且 b<c 的 c 了，可以退出循环
			if second == third {
				break
			}
			if nums[second]+nums[third] == target {
				res = append(res, []int{nums[first], nums[second], nums[third]})
			}
		}
	}
	return res
}

/* 三数之和变种 */

func threeSumTarget(nums []int, target int) [][]int {
	var res [][]int
	length := len(nums)

	// 先排序
	sort.Ints(nums)

	for i := 0; i < length; i++ {
		// 转换为两数之和
		tuples := twoSumTarget(nums, i+1, target-nums[i])
		// 拼接成三元组
		for _, tuple := range tuples {
			tuple = append(tuple, nums[i])
			res = append(res, tuple)
		}

		// 跳过第一个数字重复的情况
		for i < length-1 && nums[i] == nums[i+1] {
			i++
		}
	}

	return res
}

// 两数之和，其中nums为已排序数组
func twoSumTarget(nums []int, start, target int) [][]int {
	var res [][]int
	// 左指针改为从start开始，其他不变
	left, right := start, len(nums)-1

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

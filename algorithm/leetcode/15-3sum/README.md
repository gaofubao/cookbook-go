# 15. 三数之和

## 题目
给你一个包含 n 个整数的数组 nums，判断 nums 中是否存在三个元素 a，b，c ，使得 a + b + c = 0 ？请你找出所有和为 0 且不重复的三元组。
注意：答案中不可以包含重复的三元组。

## 思路

## 举一反三
变种：返回和目标值的三元组
```go
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
```

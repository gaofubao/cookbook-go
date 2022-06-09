package main

func fourSum(nums []int, target int) [][]int {
	var res [][]int
	length := len(nums)

	for i := 0; i < length; i++ {
		triples := threeSumTarget(nums, i+1, target-nums[i])
		for _, triple := range triples {
			triple = append(triple, nums[i])
			res = append(res, triple)
		}

		for i < length-1 && nums[i] == nums[i+1] {
			i++
		}
	}

	return res
}

// 三数之和
func threeSumTarget(nums []int, start, target int) [][]int {
	var res [][]int
	length := len(nums)

	for i := start; i < length; i++ {
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

func nSumTarget(nums []int, n, start, target int) [][]int {
	var res [][]int
	length := len(nums)

	if n < 2 || n > length {
		return res
	}

	if n == 2 {
		left, right := start, length-1
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
	} else {
		for i := start; i < length; i++ {
			sub := nSumTarget(nums, n-1, i+1, target-nums[i])
			for _, arr := range sub {
				arr = append(arr, nums[i])
				res = append(res, arr)
			}

			for i < length-1 && nums[i] == nums[i+1] {
				i++
			}
		}
	}
	return res
}

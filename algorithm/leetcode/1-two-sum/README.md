# 1. 两数之和

## 题目
给定一个整数数组 nums 和一个整数目标值 target，请你在该数组中找出和为目标值 target 的那两个整数，并返回它们的数组下标。
你可以假设每种输入只会对应一个答案。但是，数组中同一个元素在答案里不能重复出现。
你可以按任意顺序返回答案。

## 思路
- 枚举法
- 哈希表法

## 举一反三
变种一：返回和为目标值的二元组，假设每种输入只会对应一个答案
```go
// 左右指针法
func twoSum(nums []int, target int) []int {
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
            return []int{left, right}
        }
    }

    return nil
}
```

变种二：返回所有不重复的二元组
```go
func twoSum(nums []int, target int) [][]int {
    var res [][]int
    sort.Ints(nums)

    left, right := 0, len(nums)-1
    for left < right {
        leftVal, rightVal := nums[left], nums[right]
        sum := leftVal + rightVal

        if sum < target {
            left++
        } else if sum > target {
            right--
        } else {
            res = append(res, []int{left, right})
            for left < right && 
            left++
            right--
        }
    }

    return nil
}
```

## 总结
nSum问题

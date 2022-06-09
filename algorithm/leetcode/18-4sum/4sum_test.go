package main

import (
	"reflect"
	"testing"
)

func Test_fourSum(t *testing.T) {
	type args struct {
		nums   []int
		target int
	}
	tests := []struct {
		name string
		args args
		want [][]int
	}{
		{
			name: "",
			args: args{
				nums:   []int{1, 0, -1, 0, -2, 2},
				target: 0,
			},
			want: [][]int{{-2, -1, 1, 2}, {-2, 0, 0, 2}, {-1, 0, 0, 1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fourSum(tt.args.nums, tt.args.target); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fourSum() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_nSumTarget(t *testing.T) {
	type args struct {
		nums   []int
		n      int
		start  int
		target int
	}
	tests := []struct {
		name string
		args args
		want [][]int
	}{
		// {
		// 	name: "2",
		// 	args: args{
		// 		nums:   []int{2, 7, 11, 15},
		// 		n:      2,
		// 		start:  0,
		// 		target: 9,
		// 	},
		// 	want: [][]int{{2, 7}},
		// },
		{
			name: "3",
			args: args{
				nums:   []int{-1, 0, 1, 2, -1, -4},
				n:      3,
				start:  0,
				target: 0,
			},
			want: [][]int{{-1, 2, -1}, {0, 1, -1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nSumTarget(tt.args.nums, tt.args.n, tt.args.start, tt.args.target); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("nSumTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

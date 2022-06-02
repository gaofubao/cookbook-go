package main

import (
	"reflect"
	"testing"
)

func Test_twoSum3(t *testing.T) {
	type args struct {
		nums   []int
		target int
	}
	tests := []struct {
		name string
		args args
		want []int
	}{
		{
			name: "",
			args: args{
				nums:   []int{2, 7, 11, 15},
				target: 9,
			},
			want: []int{2, 7},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := twoSum3(tt.args.nums, tt.args.target); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("twoSum3() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_twoSum4(t *testing.T) {
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
				nums:   []int{2, 7, 11, 15},
				target: 9,
			},
			want: [][]int{{2, 7}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := twoSum4(tt.args.nums, tt.args.target); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("twoSum4() = %v, want %v", got, tt.want)
			}
		})
	}
}

package main

import (
	"reflect"
	"testing"
)

func Test_threeSumTarget(t *testing.T) {
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
				nums:   []int{-1, 0, 1, 2, -1, -4},
				target: 0,
			},
			want: [][]int{{-1, 2, -1}, {0, 1, -1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := threeSumTarget(tt.args.nums, tt.args.target); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("threeSumTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

package main

import "math/rand"

type Solution struct {
	nums []int
}

func Constructor(nums []int) Solution {
	return Solution{nums: nums}
}

func (s *Solution) Reset() []int {
	return append([]int(nil), s.nums...)
}

// Shuffle is Fisher-Yates from the front, the other way round than the
// Python reference.
func (s *Solution) Shuffle() []int {
	out := append([]int(nil), s.nums...)
	for i := range out {
		j := i + rand.Intn(len(out)-i)
		out[i], out[j] = out[j], out[i]
	}
	return out
}

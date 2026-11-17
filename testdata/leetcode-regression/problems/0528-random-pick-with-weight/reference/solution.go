package main

import "math/rand"

// Solution uses Vose's alias method, unlike the Python reference's prefix
// sums: each index i has a slot that it keeps with probability prob[i] and
// otherwise gives to alias[i].
type Solution struct {
	prob  []float64
	alias []int
}

func Constructor(w []int) Solution {
	n := len(w)
	total := 0
	for _, x := range w {
		total += x
	}
	prob, alias := make([]float64, n), make([]int, n)
	scaled := make([]float64, n)
	var small, large []int
	for i, x := range w {
		scaled[i] = float64(x) * float64(n) / float64(total)
		if scaled[i] < 1 {
			small = append(small, i)
		} else {
			large = append(large, i)
		}
	}
	for len(small) > 0 && len(large) > 0 {
		s, l := small[len(small)-1], large[len(large)-1]
		small = small[:len(small)-1]
		prob[s], alias[s] = scaled[s], l
		scaled[l] -= 1 - scaled[s]
		if scaled[l] < 1 {
			large = large[:len(large)-1]
			small = append(small, l)
		}
	}
	// What is left is 1 up to rounding errors.
	for _, i := range append(small, large...) {
		prob[i], alias[i] = 1, i
	}
	return Solution{prob: prob, alias: alias}
}

func (s *Solution) PickIndex() int {
	i := rand.Intn(len(s.prob))
	if rand.Float64() < s.prob[i] {
		return i
	}
	return s.alias[i]
}

package main

import "slices"

// wiggleSort is the three-way partition around the median over "virtual"
// indices 1, 3, 5, ..., 0, 2, 4, ...: larger elements fill the odd indices
// and smaller ones the even indices from the end, in partition order rather
// than sorted like the Python reference.
func wiggleSort(nums []int) {
	n := len(nums)
	sorted := slices.Clone(nums)
	slices.Sort(sorted)
	median := sorted[n/2]
	at := func(i int) int { return (1 + 2*i) % (n | 1) }
	i, j, k := 0, 0, n-1
	for j <= k {
		switch {
		case nums[at(j)] > median:
			nums[at(i)], nums[at(j)] = nums[at(j)], nums[at(i)]
			i++
			j++
		case nums[at(j)] < median:
			nums[at(j)], nums[at(k)] = nums[at(k)], nums[at(j)]
			k--
		default:
			j++
		}
	}
}

package main

import "slices"

func findMedianSortedArrays(nums1 []int, nums2 []int) float64 {
	merged := slices.Sorted(slices.Values(append(append([]int{}, nums1...), nums2...)))
	n := len(merged)
	if n%2 == 1 {
		return float64(merged[n/2])
	}
	return float64(merged[n/2-1]+merged[n/2]) / 2
}

package main

func findPeakElement(nums []int) int {
	// Climb towards the higher neighbour on the left, which finds another
	// peak than the Python reference when there are several.
	lo, hi := 0, len(nums)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if nums[mid] < nums[mid-1] {
			hi = mid - 1
		} else {
			lo = mid
		}
	}
	return lo
}

package main

func removeElement(nums []int, val int) int {
	// Moves the last element into each removed slot, so the remaining
	// elements end up in a different order than the input.
	n := len(nums)
	for i := 0; i < n; {
		if nums[i] == val {
			n--
			nums[i] = nums[n]
		} else {
			i++
		}
	}
	return n
}

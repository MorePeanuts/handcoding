package main

// sortedArrayToBST roots each range at its right middle element, so that the
// tree differs from the Python reference for ranges of even length.
func sortedArrayToBST(nums []int) *TreeNode {
	if len(nums) == 0 {
		return nil
	}
	mid := len(nums) / 2
	return &TreeNode{Val: nums[mid], Left: sortedArrayToBST(nums[:mid]), Right: sortedArrayToBST(nums[mid+1:])}
}

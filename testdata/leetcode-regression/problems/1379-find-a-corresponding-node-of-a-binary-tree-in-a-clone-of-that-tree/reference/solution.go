package main

func getTargetCopy(original, cloned, target *TreeNode) *TreeNode {
	// Values are unique, so search the clone for the target's value.
	if cloned == nil || cloned.Val == target.Val {
		return cloned
	}
	if n := getTargetCopy(nil, cloned.Left, target); n != nil {
		return n
	}
	return getTargetCopy(nil, cloned.Right, target)
}

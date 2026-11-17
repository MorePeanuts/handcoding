package main

func flatten(root *TreeNode) {
	// Morris-style: splice each left subtree in front of the right one.
	for cur := root; cur != nil; cur = cur.Right {
		if cur.Left == nil {
			continue
		}
		pre := cur.Left
		for pre.Right != nil {
			pre = pre.Right
		}
		pre.Right = cur.Right
		cur.Right, cur.Left = cur.Left, nil
	}
}

package main

func canMerge(trees []*TreeNode) *TreeNode {
	// Merge first, then check: the result must hold every tree (n - 1
	// merges) and its in-order traversal must be strictly increasing.
	roots := map[int]*TreeNode{}
	isLeaf := map[int]bool{}
	total := 0
	for _, t := range trees {
		roots[t.Val] = t
		total++
		for _, c := range []*TreeNode{t.Left, t.Right} {
			if c != nil {
				isLeaf[c.Val] = true
				total++
			}
		}
	}
	var root *TreeNode
	for _, t := range trees {
		if !isLeaf[t.Val] {
			if root != nil {
				return nil
			}
			root = t
		}
	}
	if root == nil {
		return nil
	}
	delete(roots, root.Val)
	var merge func(n *TreeNode)
	merge = func(n *TreeNode) {
		for _, c := range []**TreeNode{&n.Left, &n.Right} {
			if *c == nil {
				continue
			}
			if t, ok := roots[(*c).Val]; ok && (*c).Left == nil && (*c).Right == nil {
				delete(roots, t.Val)
				*c = t
			}
			merge(*c)
		}
	}
	merge(root)
	if len(roots) > 0 {
		return nil
	}
	count, prev, ok := 0, 0, true
	var inorder func(n *TreeNode)
	inorder = func(n *TreeNode) {
		if n == nil || !ok {
			return
		}
		inorder(n.Left)
		if count > 0 && n.Val <= prev {
			ok = false
		}
		prev = n.Val
		count++
		inorder(n.Right)
	}
	inorder(root)
	// Every merge turns a leaf and a root into one node.
	if !ok || count != total-(len(trees)-1) {
		return nil
	}
	return root
}

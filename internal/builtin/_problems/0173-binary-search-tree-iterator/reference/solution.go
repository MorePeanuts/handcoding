package main

type BSTIterator struct {
	vals []int
}

func Constructor(root *TreeNode) BSTIterator {
	// Flattens the whole in-order traversal up front.
	var it BSTIterator
	var walk func(*TreeNode)
	walk = func(n *TreeNode) {
		if n == nil {
			return
		}
		walk(n.Left)
		it.vals = append(it.vals, n.Val)
		walk(n.Right)
	}
	walk(root)
	return it
}

func (this *BSTIterator) Next() int {
	v := this.vals[0]
	this.vals = this.vals[1:]
	return v
}

func (this *BSTIterator) HasNext() bool {
	return len(this.vals) > 0
}

package main

import (
	"strconv"
	"strings"
)

// Codec writes the tree in preorder, with # for a missing child.
type Codec struct{}

func Constructor() Codec {
	return Codec{}
}

func (this *Codec) serialize(root *TreeNode) string {
	var b strings.Builder
	var walk func(n *TreeNode)
	walk = func(n *TreeNode) {
		if n == nil {
			b.WriteString("# ")
			return
		}
		b.WriteString(strconv.Itoa(n.Val))
		b.WriteByte(' ')
		walk(n.Left)
		walk(n.Right)
	}
	walk(root)
	return b.String()
}

func (this *Codec) deserialize(data string) *TreeNode {
	tokens := strings.Fields(data)
	var build func() *TreeNode
	build = func() *TreeNode {
		t := tokens[0]
		tokens = tokens[1:]
		if t == "#" {
			return nil
		}
		v, _ := strconv.Atoi(t)
		n := &TreeNode{Val: v}
		n.Left = build()
		n.Right = build()
		return n
	}
	return build()
}

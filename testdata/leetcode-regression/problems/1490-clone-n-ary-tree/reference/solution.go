package main

func cloneTree(root *Node) *Node {
	if root == nil {
		return nil
	}
	clone := &Node{Val: root.Val}
	for _, c := range root.Children {
		clone.Children = append(clone.Children, cloneTree(c))
	}
	return clone
}

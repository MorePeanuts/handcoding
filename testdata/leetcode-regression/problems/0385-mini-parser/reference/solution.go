package main

func deserialize(s string) *NestedInteger {
	// Recursive descent.
	i := 0
	var parse func() *NestedInteger
	parse = func() *NestedInteger {
		n := &NestedInteger{}
		if s[i] != '[' {
			sign, v := 1, 0
			if s[i] == '-' {
				sign = -1
				i++
			}
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				v = v*10 + int(s[i]-'0')
				i++
			}
			n.SetInteger(sign * v)
			return n
		}
		i++ // [
		for s[i] != ']' {
			n.Add(*parse())
			if s[i] == ',' {
				i++
			}
		}
		i++ // ]
		return n
	}
	return parse()
}

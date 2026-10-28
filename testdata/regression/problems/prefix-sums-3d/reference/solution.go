package main

func prefixSums(grid [][][]int) [][][]int {
	// Inclusion-exclusion over the seven smaller prefixes.
	a, b, c := len(grid), len(grid[0]), len(grid[0][0])
	s := make([][][]int, a)
	at := func(i, j, k int) int {
		if i < 0 || j < 0 || k < 0 {
			return 0
		}
		return s[i][j][k]
	}
	for i := range s {
		s[i] = make([][]int, b)
		for j := range s[i] {
			s[i][j] = make([]int, c)
			for k := range s[i][j] {
				s[i][j][k] = grid[i][j][k] +
					at(i-1, j, k) + at(i, j-1, k) + at(i, j, k-1) -
					at(i-1, j-1, k) - at(i-1, j, k-1) - at(i, j-1, k-1) +
					at(i-1, j-1, k-1)
			}
		}
	}
	return s
}

package main

func makeIntegerBeautiful(n int64, target int) int64 {
	digitSum := func(v int64) int {
		s := 0
		for ; v > 0; v /= 10 {
			s += int(v % 10)
		}
		return s
	}
	// Zero the lowest nonzero digit by carrying into the next one, until the
	// digit sum is small enough.
	m := n
	for p := int64(10); digitSum(m) > target; p *= 10 {
		if r := m % p; r != 0 {
			m += p - r
		}
	}
	return m - n
}

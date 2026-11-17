package main

func nextGreatestLetter(letters []byte, target byte) byte {
	// A linear scan: the first letter greater than target, else the first.
	for _, c := range letters {
		if c > target {
			return c
		}
	}
	return letters[0]
}

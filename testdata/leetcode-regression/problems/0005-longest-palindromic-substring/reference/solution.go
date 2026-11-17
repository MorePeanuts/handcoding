package main

// longestPalindrome keeps the rightmost longest palindrome, so that it
// differs from the Python reference whenever there is a tie ("aba" for
// "babad").
func longestPalindrome(s string) string {
	start, end := len(s)-1, len(s)
	for center := 2*len(s) - 2; center >= 0; center-- {
		lo, hi := center/2, (center+1)/2
		for lo >= 0 && hi < len(s) && s[lo] == s[hi] {
			lo--
			hi++
		}
		if hi-lo-1 > end-start {
			start, end = lo+1, hi
		}
	}
	return s[start:end]
}

# Any longest palindromic substring is accepted: "babad" has "bab" and "aba".
# expected, the reference's answer, gives the longest length.


def check(input, output, expected):
    s = input["s"]
    if not isinstance(output, str):
        return "the answer must be a string, got %r" % (output,)
    if output not in s:
        return "%r is not a substring of s" % output
    if output != output[::-1]:
        return "%r is not a palindrome" % output
    if len(output) != len(expected):
        return "%r has length %d, but the longest palindromic substring has length %d, e.g. %r" % (
            output, len(output), len(expected), expected)

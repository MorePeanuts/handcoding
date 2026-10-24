# Records docs/images/demo.gif. Run from the repository root:
#
#   vhs docs/demo/demo.tape
#
# Needs vhs, Go, Neovim and network access (the statement is fetched from
# leetcode.com).

Output docs/images/demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 720
Set Padding 16
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms

Hide
Type `eval "$(docs/demo/setup.sh)" && clear`
Enter
Sleep 15s
Show

# The problem list.
Type "hc"
Sleep 500ms
Enter
Sleep 2s

# Open Two Sum: the statement, and the solution path is copied.
Enter
Sleep 2.5s
Down 6
Sleep 1.5s

# Test: the solution has a bug.
Type "t"
Sleep 4s

# Fix it in the editor.
Type "e"
Sleep 1.5s
Type ":%s/j := i;/j := i + 1;/"
Sleep 800ms
Enter
Sleep 1.5s
Type ":wq"
Enter
Sleep 1s

# Test again, then submit.
Type "t"
Sleep 3s
Type "s"
Sleep 3s

# Submission history and the recorded code.
Type "h"
Sleep 1.5s
Enter
Sleep 3s

Escape
Sleep 300ms
Escape
Sleep 300ms
Escape
Sleep 300ms
Type "q"
Sleep 500ms

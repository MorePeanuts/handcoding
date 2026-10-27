package judge

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestChecked(t *testing.T) {
	ok := CheckerResult{ID: 1, OK: true}
	rejected := CheckerResult{ID: 2, Message: "not a palindrome"}
	got, err := Checked([]int{1, 2}, CheckerRun{Path: "checker.py", Results: []CheckerResult{ok, rejected}})
	if err != nil || !got[1].OK || got[2].OK || got[2].Message != "not a palindrome" {
		t.Fatalf("Checked = %+v, %v", got, err)
	}
	for _, tt := range []struct {
		run  CheckerRun
		want string
	}{
		{CheckerRun{Results: []CheckerResult{{ID: 0, Error: "SyntaxError: invalid syntax"}}}, "checker.py cannot be loaded (the problem's checker is broken; this is not a verdict on your solution)\nSyntaxError"},
		{CheckerRun{Results: []CheckerResult{ok, {ID: 2, Error: "KeyError: 'nums'"}}}, "checker.py failed on case 2 (the problem's checker is broken; this is not a verdict on your solution)\nKeyError"},
		{CheckerRun{Results: []CheckerResult{ok}, TimedOut: true, Limit: 2 * time.Second}, "failed on case 2: time limit of 2s exceeded"},
		{CheckerRun{Results: []CheckerResult{ok}, Output: "Killed"}, "failed on case 2: exited without a result (the problem's checker is broken; this is not a verdict on your solution)\nKilled"},
	} {
		tt.run.Path = "checker.py"
		_, err := Checked([]int{1, 2}, tt.run)
		if _, isChecker := errors.AsType[*CheckerError](err); !isChecker || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Checked(%+v) error = %v, want %q", tt.run, err, tt.want)
		}
	}
}

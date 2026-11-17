package javascript_test

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/testenv"
)

func TestNumericTokens(t *testing.T) {
	t.Parallel()
	node := testenv.LookPath(t, "node")
	codec, err := filepath.Abs("runtime/codec.cjs")
	if err != nil {
		t.Fatal(err)
	}
	// Cover native integer parsing and source-aware parsing, including values
	// that JSON.parse rounds to an otherwise valid integer.
	script := `const assert = require('node:assert/strict');
const {parse, decode} = require(process.argv[1]);
for (const text of ['0', '-2147483648', '2147483647', '999999999999999', '9007199254740991', '-9007199254740991'])
  assert.equal(decode('long', parse(text)), Number(text));
for (const text of ['9007199254740993', '-9007199254740993', '1.0000000000000001', '1e100'])
  assert.throws(() => decode('long', parse(text)));
assert.equal(decode('double', parse('1.25')), 1.25);
assert.equal(decode('double', parse('1e-3')), 0.001);
assert.deepEqual(decode('integer[][]', parse('[[1,-2],[]]')), [[1,-2],[]]);
assert.deepEqual(parse('["12345678901234567890", "1e3", "1.2", "get", 1]'), ['12345678901234567890', '1e3', '1.2', 'get', 1]);
const nested = decode('NestedInteger', parse('[1,[2]]'));
assert.equal(nested.getList()[0].getInteger(), 1);
assert.equal(nested.getList()[1].getList()[0].getInteger(), 2);`
	if out, err := exec.Command(node, "-e", script, codec).CombinedOutput(); err != nil {
		t.Fatalf("numeric codecs: %v\n%s", err, out)
	}
}

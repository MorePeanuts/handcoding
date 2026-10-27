package problem

import "testing"

func TestMarshalCase(t *testing.T) {
	tests := []struct{ line, want string }{
		// Compact, keys in a fixed order, & < > written as they are.
		{`{"note":"k > n & m < 3", "example":true, "expected":"a&b<c>", "input":{"s":"a&b<c>","k":[ 1, 2 ]}}`,
			`{"input":{"k":[1,2],"s":"a&b<c>"},"expected":"a&b<c>","example":true,"note":"k > n & m < 3"}`},
		// Escapes written by Go (&) and Python (é) are undone,
		// number literals are kept.
		{`{"input":{"s":"a&b é你","x":[1.50,1e3,-0]},"expected":"\"\n\u0001"}`,
			`{"input":{"s":"a&b é你","x":[1.50,1e3,-0]},"expected":"\"\n\u0001"}`},
		{`{"input":{"root":null},"expected":null}`, `{"input":{"root":null},"expected":null}`},
		{`{"input":{"x":1}}`, `{"input":{"x":1}}`},
	}
	for _, tt := range tests {
		got, err := MarshalCase(mustParseCase(t, tt.line))
		if err != nil {
			t.Fatalf("%s: %v", tt.line, err)
		}
		if string(got) != tt.want {
			t.Errorf("MarshalCase(%s)\n got %s\nwant %s", tt.line, got, tt.want)
		}
	}
}

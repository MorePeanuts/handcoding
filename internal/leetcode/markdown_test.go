package leetcode

import (
	"reflect"
	"strings"
	"testing"

	"github.com/MorePeanuts/handcoding/internal/problem"
)

func markdownImages(md string) []string {
	var images []string
	problem.ReplaceImages(md, func(_, src string) string { images = append(images, src); return "" })
	return images
}

func TestPreImages(t *testing.T) {
	for _, tt := range []struct {
		name, html       string
		sequence, images []string
	}{
		{"mixed", `<img src="/outside.png"><pre><strong>Input:</strong> x = 1
<strong>Output:</strong> 1
<strong>Explanation:</strong> Before
<img alt="diagram" src="/inside.png">After
</pre>`, []string{"/outside.png", "Before", "![diagram](/inside.png)", "After"}, []string{"/outside.png", "/inside.png"}},
		{"multiple nested", `<pre class="language-text"><code>Before<span>Left<img src="/a.png">Middle<img src="/b.png">Right</span>After</code></pre>`, []string{"BeforeLeft", "/a.png", "Middle", "/b.png", "RightAfter"}, []string{"/a.png", "/b.png"}},
		{"image only", `<pre><img src="/a.png"><img src="/b.png"></pre>`, []string{"/a.png", "/b.png"}, []string{"/a.png", "/b.png"}},
		{"escaped HTML is code", `<pre><code>&lt;img src="/literal.png"&gt;
    ![](/also-literal.png)
</code></pre>`, []string{`<img src="/literal.png">`, "    ![](/also-literal.png)"}, nil},
		{"code", "<pre><code class=\"language-python\">if x &lt; 2:\n    return x\n\n# ``` stays text\n</code></pre>", []string{"````python", "if x < 2:\n    return x\n\n# ``` stays text", "````"}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			md, err := toMarkdown(tt.html)
			if err != nil {
				t.Fatal(err)
			}
			if got := markdownImages(md); !reflect.DeepEqual(got, tt.images) {
				t.Fatalf("images=%v want=%v\n%s", got, tt.images, md)
			}
			rest := md
			for _, part := range tt.sequence {
				i := strings.Index(rest, part)
				if i < 0 {
					t.Fatalf("missing/out of order %q:\n%s", part, md)
				}
				rest = rest[i+len(part):]
			}
		})
	}
}

// Captured English statements include alternative-answer diagrams inside pre;
// their Chinese translations may put images elsewhere and share assets.
func TestCapturedStatementImages(t *testing.T) {
	for _, tt := range statementImageFixtures {
		t.Run(tt.slug, func(t *testing.T) {
			c, err := Convert(loadQuestion(t, tt.slug), SiteCN)
			if err != nil {
				t.Fatal(err)
			}
			got := markdownImages(c.Statements[LocaleEnglish])
			if len(got) != len(tt.images) {
				t.Fatalf("images=%v want=%v", got, tt.images)
			}
			for i, name := range tt.images {
				if !strings.HasSuffix(got[i], "/"+name) {
					t.Errorf("image %d=%s want %s", i, got[i], name)
				}
			}
		})
	}
}

var statementImageFixtures = []struct {
	slug   string
	images []string
}{
	{"insert-into-a-binary-search-tree", []string{"insertbst.jpg", "bst.jpg"}},
	{"delete-node-in-a-bst", []string{"del_node_1.jpg", "del_node_supp.jpg"}},
	{"convert-sorted-array-to-binary-search-tree", []string{"btree1.jpg", "btree2.jpg", "btree.jpg"}},
}

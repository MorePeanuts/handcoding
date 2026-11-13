package leetcode

import (
	"regexp"
	"slices"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"golang.org/x/net/html"
)

var supRE = regexp.MustCompile(`<sup>(.*?)</sup>`)

func toMarkdown(content string) (string, error) {
	// html-to-markdown drops <sup>; keep exponents readable (10^4).
	content = supRE.ReplaceAllString(content, "^$1")
	content = strings.ReplaceAll(content, "&nbsp;", " ")
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return "", err
	}
	splitPreImages(doc)
	md, err := htmltomarkdown.ConvertNode(doc)
	return string(md), err
}

// The converter renders pre as code by collecting only text nodes, losing
// nested images. Lift each image out, splitting its ancestors into the text
// before and after it. Both halves retain their pre/code attributes and
// whitespace, and the image reaches the converter's normal image handler.
func splitPreImages(doc *html.Node) {
	var images []*html.Node
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			images = append(images, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(doc)
	for _, img := range images {
		pre := img.Parent
		for pre != nil && (pre.Type != html.ElementNode || pre.Data != "pre") {
			pre = pre.Parent
		}
		if pre == nil {
			continue
		}
		container := pre.Parent
		for img.Parent != container {
			parent := img.Parent
			after := &html.Node{Type: parent.Type, DataAtom: parent.DataAtom, Data: parent.Data, Namespace: parent.Namespace, Attr: slices.Clone(parent.Attr)}
			for img.NextSibling != nil {
				next := img.NextSibling
				parent.RemoveChild(next)
				after.AppendChild(next)
			}
			parent.RemoveChild(img)
			parent.Parent.InsertBefore(img, parent.NextSibling)
			if after.FirstChild != nil {
				img.Parent.InsertBefore(after, img.NextSibling)
			}
			if parent.FirstChild == nil {
				parent.Parent.RemoveChild(parent)
			}
		}
	}
}

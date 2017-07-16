package main

import (
	"bytes"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var (
	htmlTagRegexp     = regexp.MustCompile(`<[^>]*>`)
	htmlTagNameRegexp = regexp.MustCompile(`<(\S+)[^>]*>`)
	htmlAttrRegexp    = regexp.MustCompile(`(\S+)=["']([^"']+)["']`)
	htmlNoteImgRegexp = regexp.MustCompile(`(<img src=")[^!]*!EN-RESOURCE\|([0-9a-fA-F]+)\+([^!]+)!\s*([^"]*)("[^>]*>)`)
)

func NoteToMarkdown(content string, hash2guid map[string]string) (string, error) {
	buf := bytes.NewBuffer(nil)

	// Parse content
	root, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return "", err
	}

	// Travse nodes
	var f func(node *html.Node)
	f = func(n *html.Node) {

		if n.Type == html.TextNode {
			buf.WriteString(n.Data)
		} else if n.Type == html.ElementNode {
			if n.Data == "en-media" {
				hash := ""
				for _, attr := range n.Attr {
					if attr.Key == "hash" {
						hash = attr.Val
						break
					}
				}

				if hash2guid != nil {
					if guid, ok := hash2guid[hash]; ok {
						buf.WriteString(`!EN-RESOURCE|` + guid + `!`)
					} else {
						buf.WriteString(`!EN-RESOURCE-HASH|` + hash + `!`)
					}
				} else {
					buf.WriteString(`!EN-RESOURCE-HASH|` + hash + `!`)
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}

		if n.Type == html.ElementNode && n.DataAtom == atom.Div {
			buf.WriteString("\r\n")
		}
	}
	f(root)

	result := buf.String()

	// Remove no-break space
	const nbsp = rune(0x20)
	result = strings.Map(func(r rune) rune {
		if r == 0xa0 {
			return nbsp
		}
		return r
	}, result)

	return result, nil
}

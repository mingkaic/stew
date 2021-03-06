//// file: stew.go

// Package stew ...
// Is a lightweight extensible web scraping package
package stew

import (
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/html"
	"gopkg.in/eapache/queue.v1"
)

// =============================================
//                    Declarations
// =============================================

type DescMap map[string]map[*Stew]struct{}

// Stew ...
// Is a queryable alternative to html.Node
type Stew struct {
	// Breadth-first position of element
	Pos uint
	// Tag name of current node
	Tag string
	// Pointer to parent node
	Parent *Stew
	// Pointers to children node
	Children []*Stew
	// Descs maps descendent tag name to Stew nodes
	Descs DescMap // discarding order information for searchability
	// Attrs ... map attribute key to value
	// empty string attrs key is the text content
	Attrs map[string][]string
}

// ElemLookup ...
// Is a functor type for DOM-tree BFS
type ElemLookup func(*html.Node) []*html.Node

// functor determines whether input node is a target
// and whether it terminates the DOM search
type queryOpt func(*html.Node) bool

// =============================================
//                    Public
// =============================================

//// Creator & Members for Stew Node

// New ...
// Visits link and extracts the Stew tree representation of the static DOM
func New(link string) *Stew {
	resp, err := http.Get(link)
	if err != nil {
		panic(err)
	}
	return NewFromRes(resp)
}

// NewFromRes ...
// Parses input response and returns the Stew tree root
func NewFromRes(res *http.Response) *Stew {
	return NewFromReader(res.Body)
}

// NewFromReader ...
// Parses input html reader source and returns the Stew tree root
func NewFromReader(body io.ReadCloser) *Stew {
	defer body.Close()
	root, err := html.Parse(body)
	if err != nil {
		panic(err)
	}
	return NewFromNode(root)
}

// NewFromNode ...
// Traverses through input root node and returns the Stew tree root
func NewFromNode(root *html.Node) *Stew {
	// parse root
	type nodePair struct {
		h *html.Node
		s *Stew
	}
	downQueue := queue.New()
	upQueue := queue.New()
	upVisits := make(map[*Stew]uint)

	// propagate down the tree collecting immediate descendants
	result := &Stew{Pos: 0, Tag: root.Data,
		Descs: make(DescMap),
		Attrs: make(map[string][]string)}
	downQueue.Add(nodePair{root, result})
	var pos uint = 1

	for downQueue.Length() > 0 {
		curr := downQueue.Peek().(nodePair)
		downQueue.Remove()
		hNode := curr.h
		sNode := curr.s

		for _, attr := range hNode.Attr {
			sNode.Attrs[attr.Key] = append(sNode.Attrs[attr.Key], attr.Val)
		}
		for child := hNode.FirstChild; child != nil; child = child.NextSibling {
			switch child.Type {
			case html.ElementNode:
				upVisits[sNode]++
				sChild := &Stew{Pos: pos, Tag: child.Data,
					Descs:  make(DescMap),
					Attrs:  make(map[string][]string),
					Parent: sNode}
				pos++
				sNode.Children = append(sNode.Children, sChild)
				descs, ok := sNode.Descs[child.Data]
				if !ok {
					descs = make(map[*Stew]struct{})
					sNode.Descs[child.Data] = descs
				}
				descs[sChild] = struct{}{}
				downQueue.Add(nodePair{child, sChild})
			case html.TextNode:
				content := strings.TrimSpace(child.Data)
				if len(content) > 0 {
					sNode.Attrs[""] = append(sNode.Attrs[""], content)
				}
			}
		}
		if len(sNode.Descs) == 0 {
			upQueue.Add(sNode) // add leaves
		}
	}

	// propagate up the tree merging descendant maps
	for upQueue.Length() > 0 {
		curr := upQueue.Peek().(*Stew)
		upQueue.Remove()
		// push diff(curr.Desc, curr.Parent.Desc) to curr.Parent.Desc
		for _, child := range curr.Children {
			for key, value := range child.Descs {
				if descs, ok := curr.Descs[key]; ok {
					// merge value and descs
					for v := range value {
						descs[v] = struct{}{}
					}
				} else {
					curr.Descs[key] = value
				}
			}
		}

		upVisits[curr.Parent]--
		if curr.Parent != nil && upVisits[curr.Parent] == 0 {
			upQueue.Add(curr.Parent)
		}
	}

	return result
}

// FindAll ...
// Returns all Stew nodes matching input tags
func (this *Stew) FindAll(tags ...string) []*Stew {
	stews := make(map[*Stew]struct{})
	for _, tag := range tags {
		if this.Tag == tag {
			stews[this] = struct{}{}
			break
		}
	}

	for _, tag := range tags {
		if desc, ok := this.Descs[tag]; ok {
			for v := range desc {
				stews[v] = struct{}{}
			}
		}
	}
	slist := make([]*Stew, 0, len(stews))
	for s := range stews {
		slist = append(slist, s)
	}
	results := make([]*Stew, len(slist))
	for i, tag := range slist {
		results[i] = tag
	}
	return results
}

// Find ...
// Returns all Stew nodes with matching input attr key-val pair
func (this *Stew) Find(attrKey, attrVal string) []*Stew {
	results := []*Stew{}
	for _, attrVal := range this.Attrs[attrKey] {
		if attrVal == attrVal {
			results = append(results, this)
			break
		}
	}

	for _, stews := range this.Descs {
		for s := range stews {
			for _, val := range s.Attrs[attrKey] {
				if val == attrVal {
					results = append(results, s)
					break
				}
			}
		}
	}
	return results
}

//// Quick Lookups

// FindAll ...
// Returns functor looking for elements with input tags
func FindAll(tags ...string) ElemLookup {
	return generateLookup(
		func(node *html.Node) bool {
			isTarget := false
			for _, tag := range tags {
				isTarget = isTarget || node.Data == tag
			}
			return isTarget
		})
}

// Find ...
// Returns functor looking for elements matching input attr key-val pair
func Find(attrKey, attrVal string) ElemLookup {
	return generateLookup(
		func(node *html.Node) bool {
			for _, attr := range node.Attr {
				if attr.Key == attrKey {
					return attr.Val == attrVal
				}
			}
			return false
		})
}

// =============================================
//                    Private
// =============================================

// generates a breadth first DOM search given a query functor
func generateLookup(query queryOpt) ElemLookup {
	return func(root *html.Node) []*html.Node {
		results := []*html.Node{}
		queue := queue.New()
		queue.Add(root)

		for queue.Length() > 0 {
			curr := queue.Peek().(*html.Node)
			queue.Remove()
			if query(curr) {
				results = append(results, curr)
			}

			for child := curr.FirstChild; child != nil; child = child.NextSibling {
				queue.Add(child)
			}
		}

		return results
	}
}

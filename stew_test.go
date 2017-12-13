//// file: stew_test.go

package stew

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"testing"

	"golang.org/x/net/html"
	"gopkg.in/eapache/queue.v1"
	"gopkg.in/fatih/set.v0"
)

//// Utility structures

type mockRC struct {
	*bytes.Buffer
}

func (rc *mockRC) Close() (err error) {
	return
}

type dErr struct {
	expectPos           uint
	prefix, expect, got string
}

type groupPair struct {
	args []string
	res  *set.Set
}

//// Globals

const sampleHtml = `
<html>
	<head>
		<title>your title here</title>
	</head>
	<body bgcolor="ffffff">
		<center><img src="clouds.jpg" align="bottom"></center>
		<hr>
		<a href="http://somegreatsite.com">link name</a>
		is a link to another nifty site
		<h1>this is a header</h1>
		<h2>this is a medium header</h2>
		send me mail at
		<a href="mailto:support@yourcompany.com">support@yourcompany.com</a>.
		<p>this is a new paragraph!
		<p>
			<b>this is a new paragraph!</b>
			<br>
			<b><i>this is a new sentence without a paragraph break, in bold italics.</i></b>
		<hr>
	</body>
</html>
`

var expectedStew *Stew

var expectedTagGroup []groupPair

var expectedAttrGroup []groupPair

//// Tests

func TestMain(m *testing.M) {
	setupExpectation()
	retCode := m.Run()
	os.Exit(retCode)
}

func TestNew(t *testing.T) {
	var rc io.ReadCloser = &mockRC{bytes.NewBufferString(sampleHtml)}
	stewie := New(rc)

	diffs := utilStewDiff(expectedStew, stewie)
	for _, diff := range diffs {
		t.Errorf("@%d '%s' (expected %s, got %s)\n",
			diff.expectPos, diff.prefix,
			diff.expect, diff.got)
	}
}

func TestFindAll(t *testing.T) {
	var rc io.ReadCloser = &mockRC{bytes.NewBufferString(sampleHtml)}
	stewie := New(rc)

	for _, gp := range expectedTagGroup {
		group := stewie.FindAll(gp.args...)
		if gp.res.Size() != len(group) {
			t.Errorf("expecting tag group of size %d, got %d", gp.res.Size(), len(group))
		} else {
			expArr := make([]int, gp.res.Size())
			for i, elem := range gp.res.List() {
				expArr[i] = int(elem.(*Stew).Pos)
			}
			gotArr := make([]int, len(group))
			for i, st := range group {
				gotArr[i] = int(st.Pos)
			}
			sort.Ints(expArr)
			sort.Ints(gotArr)
			if !reflect.DeepEqual(expArr, gotArr) {
				t.Errorf("expecting tag group %s, got %s", expArr, gotArr)
			}
		}
	}
}

func TestQuickFindAll(t *testing.T) {
	var rc io.ReadCloser = &mockRC{bytes.NewBufferString(sampleHtml)}
	defer rc.Close()
	root, err := html.Parse(rc)
	if err != nil {
		panic(err)
	}

	for _, gp := range expectedTagGroup {
		group := FindAll(gp.args...)(root)
		if gp.res.Size() != len(group) {
			t.Errorf("expecting tag group of size %d, got %d", gp.res.Size(), len(group))
		} else {
			expArr := make([]string, gp.res.Size())
			for i, elem := range gp.res.List() {
				expArr[i] = elem.(*Stew).Tag
			}
			gotArr := make([]string, len(group))
			for i, n := range group {
				gotArr[i] = n.Data
			}
			sort.Strings(expArr)
			sort.Strings(gotArr)
			if !reflect.DeepEqual(expArr, gotArr) {
				t.Errorf("expecting tag group %s, got %s", expArr, gotArr)
			}
		}
	}
}

func TestFind(t *testing.T) {
	var rc io.ReadCloser = &mockRC{bytes.NewBufferString(sampleHtml)}
	stewie := New(rc)

	for _, gp := range expectedAttrGroup {
		elem := stewie.Find(gp.args[0], gp.args[1])[0]
		expect := gp.res.List()[0].(*Stew)
		if expect.Pos != elem.Pos {
			t.Errorf("given attribute pairs: <%s, %s> expect <%d, %s>, got <%d, %s>",
				gp.args[0], gp.args[1], expect.Pos, expect.Tag, elem.Pos, elem.Tag)
		}
	}
}

func TestQuickFind(t *testing.T) {
	var rc io.ReadCloser = &mockRC{bytes.NewBufferString(sampleHtml)}
	defer rc.Close()
	root, err := html.Parse(rc)
	if err != nil {
		panic(err)
	}

	for _, gp := range expectedAttrGroup {
		elem := Find(gp.args[0], gp.args[1])(root)[0]
		expect := gp.res.List()[0].(*Stew)
		if expect.Tag != elem.Data {
			t.Errorf("expecting tag %s, got %s", expect.Tag, elem.Data)
		}
	}
}

//// Test utilities

func utilStewNew(pos uint, tag string, attrs map[string][]string,
	descs map[string]*set.Set, children ...*Stew) *Stew {
	stewie := &Stew{Pos: pos, Tag: tag, Attrs: attrs, Descs: descs, Children: children}
	for _, child := range children {
		child.Parent = stewie
	}
	return stewie
}

func utilStewDiff(stew1, stew2 *Stew) []dErr {
	type stewPairs struct {
		s1, s2 *Stew
	}
	result := []dErr{}
	queue := queue.New()
	queue.Add(stewPairs{stew1, stew2})

	for queue.Length() > 0 {
		sp := queue.Peek().(stewPairs)
		queue.Remove()
		// compare s1 and s2 in sp
		result = append(result, utilStewFragDiff(sp.s1, sp.s2)...)

		nChildren := len(sp.s1.Children)
		if nChildren != len(sp.s2.Children) {
			result = append(result, dErr{
				expectPos: sp.s2.Pos,
				prefix:    "different # of children",
				expect:    fmt.Sprint(len(sp.s1.Children)),
				got:       fmt.Sprint(len(sp.s2.Children)),
			})
		} else {
			for i := 0; i < nChildren; i++ {
				if stew1.Parent == nil {
					if stew2.Parent != nil {
						result = append(result, dErr{
							expectPos: stew1.Pos,
							prefix:    "different parents",
							expect:    "<nil>",
							got:       stew2.Parent.Tag,
						})
					}
				} else {
					if sp.s2.Children[i].Parent != nil {
						result = append(result, dErr{
							expectPos: stew1.Pos,
							prefix:    "different parents",
							expect:    stew1.Parent.Tag,
							got:       "<nil>",
						})
					} else if sp.s1.Children[i].Parent.Tag != sp.s2.Children[i].Parent.Tag {
						result = append(result, dErr{
							expectPos: stew1.Pos,
							prefix:    "different parents",
							expect:    stew1.Parent.Tag,
							got:       stew2.Parent.Tag,
						})
					}
				}
				queue.Add(stewPairs{sp.s1.Children[i], sp.s2.Children[i]})
			}
		}
	}

	return result
}

func utilStewFragDiff(stew1, stew2 *Stew) []dErr {
	result := []dErr{}
	if stew1.Pos != stew2.Pos {
		result = append(result, dErr{
			expectPos: stew1.Pos,
			prefix:    "different positions",
			expect:    fmt.Sprint(stew1.Pos),
			got:       fmt.Sprint(stew2.Pos),
		})
	}
	if stew1.Tag != stew2.Tag {
		result = append(result, dErr{
			expectPos: stew1.Pos,
			prefix:    "different tags",
			expect:    stew1.Tag,
			got:       stew2.Tag,
		})
	}
	if !reflect.DeepEqual(stew1.Attrs, stew2.Attrs) {
		a1, err := json.Marshal(stew1.Attrs)
		if err != nil {
			panic(err)
		}
		a2, err := json.Marshal(stew2.Attrs)
		if err != nil {
			panic(err)
		}
		result = append(result, dErr{
			expectPos: stew1.Pos,
			prefix:    "different attribute maps",
			expect:    string(a1),
			got:       string(a2),
		})
	}
	if !utilDescEqual(stew1.Descs, stew2.Descs) {
		result = append(result, dErr{
			expectPos: stew1.Pos,
			prefix:    "different descendents maps",
			expect:    utilDescString(stew1.Descs),
			got:       utilDescString(stew2.Descs),
		})
	}
	return result
}

func utilDescEqual(desc1, desc2 map[string]*set.Set) bool {
	if len(desc1) != len(desc2) {
		return false
	}
	for key, value := range desc1 {
		v2 := desc2[key]
		if v2 == nil || value.Size() != v2.Size() {
			return false
		}
		origPSet := set.New()
		value.Each(func(elem interface{}) bool {
			origPSet.Add(elem.(*Stew).Pos)
			return true
		})

		equal := true
		v2.Each(func(elem interface{}) bool {
			equal = equal && origPSet.Has(elem.(*Stew).Pos)
			return equal
		})
		// equal by pigeon hole
		if !equal {
			return false
		}
	}
	return true
}

func utilDescString(desc map[string]*set.Set) string {
	printable := make(map[string][]string)
	for key, value := range desc {
		vlist := value.List()
		printable[key] = make([]string, len(vlist))
		for i, dStew := range vlist {
			printable[key][i] = fmt.Sprint(dStew.(*Stew).Pos)
		}
	}
	result, err := json.Marshal(printable)
	if err != nil {
		panic(err)
	}
	return string(result)
}

//// Setup

// todo: serialize this for regression testing (AFTER this test and serialization test)
// hard code DOM
func setupExpectation() {
	// leaves
	title := utilStewNew(4, "title", map[string][]string{"": {"your title here"}},
		map[string]*set.Set{})
	img := utilStewNew(14, "img", map[string][]string{
		"src":   {"clouds.jpg"},
		"align": {"bottom"},
	}, map[string]*set.Set{})
	hr1 := utilStewNew(6, "hr", map[string][]string{}, map[string]*set.Set{})
	a1 := utilStewNew(7, "a", map[string][]string{
		"":     {"link name"},
		"href": {"http://somegreatsite.com"},
	}, map[string]*set.Set{})
	h1 := utilStewNew(8, "h1", map[string][]string{"": {"this is a header"}},
		map[string]*set.Set{})
	h2 := utilStewNew(9, "h2", map[string][]string{"": {"this is a medium header"}},
		map[string]*set.Set{})
	a2 := utilStewNew(10, "a", map[string][]string{
		"":     {"support@yourcompany.com"},
		"href": {"mailto:support@yourcompany.com"},
	}, map[string]*set.Set{})
	p1 := utilStewNew(11, "p", map[string][]string{"": {"this is a new paragraph!"}},
		map[string]*set.Set{})
	b1 := utilStewNew(15, "b", map[string][]string{"": {"this is a new paragraph!"}},
		map[string]*set.Set{})
	br := utilStewNew(16, "br", map[string][]string{}, map[string]*set.Set{})
	i := utilStewNew(18, "i", map[string][]string{
		"": {"this is a new sentence without a paragraph break, in bold italics."},
	}, map[string]*set.Set{})
	hr2 := utilStewNew(13, "hr", map[string][]string{}, map[string]*set.Set{})

	b2 := utilStewNew(17, "b", map[string][]string{}, map[string]*set.Set{"i": set.New(i)}, i)

	p2 := utilStewNew(12, "p", map[string][]string{}, map[string]*set.Set{
		"b":  set.New(b1, b2),
		"br": set.New(br),
		"i":  set.New(i),
	}, b1, br, b2)

	center := utilStewNew(5, "center", map[string][]string{},
		map[string]*set.Set{"img": set.New(img)}, img)

	head := utilStewNew(2, "head", map[string][]string{},
		map[string]*set.Set{"title": set.New(title)}, title)
	body := utilStewNew(3, "body", map[string][]string{
		"":        {"is a link to another nifty site", "send me mail at", "."},
		"bgcolor": {"ffffff"},
	}, map[string]*set.Set{
		"a":      set.New(a1, a2),
		"b":      set.New(b1, b2),
		"br":     set.New(br),
		"center": set.New(center),
		"h1":     set.New(h1),
		"h2":     set.New(h2),
		"hr":     set.New(hr1, hr2),
		"i":      set.New(i),
		"img":    set.New(img),
		"p":      set.New(p1, p2),
	}, center, hr1, a1, h1, h2, a2, p1, p2, hr2)

	html := utilStewNew(1, "html", map[string][]string{}, map[string]*set.Set{
		"a":      set.New(a1, a2),
		"b":      set.New(b1, b2),
		"body":   set.New(body),
		"br":     set.New(br),
		"center": set.New(center),
		"h1":     set.New(h1),
		"h2":     set.New(h2),
		"head":   set.New(head),
		"hr":     set.New(hr1, hr2),
		"i":      set.New(i),
		"img":    set.New(img),
		"p":      set.New(p1, p2),
		"title":  set.New(title),
	}, head, body)

	expectedStew = utilStewNew(0, "", map[string][]string{}, map[string]*set.Set{
		"a":      set.New(a1, a2),
		"b":      set.New(b1, b2),
		"body":   set.New(body),
		"br":     set.New(br),
		"center": set.New(center),
		"h1":     set.New(h1),
		"h2":     set.New(h2),
		"head":   set.New(head),
		"hr":     set.New(hr1, hr2),
		"html":   set.New(html),
		"i":      set.New(i),
		"img":    set.New(img),
		"p":      set.New(p1, p2),
		"title":  set.New(title),
	}, html)

	expectedTagGroup = []groupPair{
		groupPair{[]string{"a", "b", "body", "br", "center"},
			set.New(a1, a2, b1, b2, body, br, center)},
		groupPair{[]string{"h1", "h2", "head", "hr", "html"},
			set.New(h1, h2, head, hr1, hr2, html)},
		groupPair{[]string{"i", "img", "p", "title"},
			set.New(i, img, p1, p2, title)},
	}

	expectedAttrGroup = []groupPair{
		groupPair{[]string{"bgcolor", "ffffff"}, set.New(body)},
		groupPair{[]string{"src", "clouds.jpg"}, set.New(img)},
		groupPair{[]string{"align", "bottom"}, set.New(img)},
		groupPair{[]string{"align", "bottom"}, set.New(img)},
		groupPair{[]string{"href", "http://somegreatsite.com"}, set.New(a1)},
		groupPair{[]string{"href", "mailto:support@yourcompany.com"}, set.New(a2)},
	}
}

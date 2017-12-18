//// file: stew_test.go

package stew

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/mingkaic/gardener"
	"golang.org/x/net/html"
)

//// ====== Globals ======

const nTagGroup = 4

var expectedPage *gardener.HTMLNode

var sampleHTML string

var expectedTags []struct {
	args []string
	out  []*gardener.HTMLNode
}

var expectedAttrs []struct {
	attr, val string
	out       *gardener.HTMLNode
}

//// ====== Tests ======

func TestMain(m *testing.M) {
	setupExpectation()
	retCode := m.Run()
	os.Exit(retCode)
}

// TestNew ...
// Ensures scraper tree is equal to expected tree
func TestNew(t *testing.T) {
	var rc io.ReadCloser = &gardener.MockRC{bytes.NewBufferString(sampleHTML)}
	stewie := New(rc)

	treeCheck(expectedPage, stewie,
		func(msg string, args ...interface{}) {
			t.Errorf(msg, args...)
		})
}

// TestFindAll ...
// Validates Stew.FindAll function
func TestFindAll(t *testing.T) {
	var rc io.ReadCloser = &gardener.MockRC{bytes.NewBufferString(sampleHTML)}
	stewie := New(rc)

	for _, gp := range expectedTags {
		group := stewie.FindAll(gp.args...)
		if len(gp.out) != len(group) {
			t.Errorf("expecting tag group of size %d, got %d", len(gp.out), len(group))
		} else {
			expArr := make([]int, len(gp.out))
			for i, elem := range gp.out {
				expArr[i] = int(elem.Pos)
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

// TestFind ...
// Validates Stew.Find function
func TestFind(t *testing.T) {
	var rc io.ReadCloser = &gardener.MockRC{bytes.NewBufferString(sampleHTML)}
	stewie := New(rc)

	for _, gp := range expectedAttrs {
		elems := stewie.Find(gp.attr, gp.val)
		if len(elems) == 0 {
			t.Errorf("given attribute pairs: <%s, %s> expect <%d %s>, got <nil",
				gp.attr, gp.val, gp.out.Pos, gp.out.Tag)
		} else {
			if gp.out.Pos != elems[0].Pos {
				t.Errorf("given attribute pairs: <%s, %s> expect %d, got %d",
					gp.attr, gp.val, gp.out.Pos, elems[0].Pos)
			}
			if gp.out.Tag != elems[0].Tag {
				t.Errorf("given attribute pairs: <%s, %s> expect %s, got %s",
					gp.attr, gp.val, gp.out.Tag, elems[0].Tag)
			}
		}
	}

	// lookup attributes that can't exist in stewie
	elems := stewie.Find("invalid", "attribute")
	if len(elems) > 0 {
		t.Errorf("stew tree contains junk attributes")
	}
}

// TestQuickFindAll ...
// Validates FindAll closure
func TestQuickFindAll(t *testing.T) {
	var rc io.ReadCloser = &gardener.MockRC{bytes.NewBufferString(sampleHTML)}
	defer rc.Close()
	root, err := html.Parse(rc)
	check(err)

	for _, gp := range expectedTags {
		group := FindAll(gp.args...)(root)
		if len(gp.out) != len(group) {
			t.Errorf("expecting tag group of size %d, got %d", len(gp.out), len(group))
		} else {
			expArr := make([]string, len(gp.out))
			for i, elem := range gp.out {
				expArr[i] = elem.Tag
			}
			gotArr := make([]string, len(group))
			for i, st := range group {
				gotArr[i] = st.Data
			}
			sort.Strings(expArr)
			sort.Strings(gotArr)
			if !reflect.DeepEqual(expArr, gotArr) {
				t.Errorf("expecting tag group %s, got %s", expArr, gotArr)
			}
		}
	}
}

// TestQuickFind ...
// Validates Find closure
func TestQuickFind(t *testing.T) {
	var rc io.ReadCloser = &gardener.MockRC{bytes.NewBufferString(sampleHTML)}
	defer rc.Close()
	root, err := html.Parse(rc)
	check(err)

	for _, gp := range expectedAttrs {
		elems := Find(gp.attr, gp.val)(root)
		if len(elems) == 0 {
			t.Errorf("given attribute pairs: <%s, %s> expect %s, got <nil",
				gp.attr, gp.val, gp.out.Tag)
		} else {
			if gp.out.Tag != elems[0].Data {
				t.Errorf("given attribute pairs: <%s, %s> expect %s, got %s",
					gp.attr, gp.val, gp.out.Tag, elems[0].Data)
			}
		}
	}

	// lookup attributes that can't exist in stewie
	elems := Find("invalid", "attribute")(root)
	if len(elems) > 0 {
		t.Errorf("stew tree contains junk attributes")
	}
}

//// ====== Utilities ======

//// Core Utilities

func treeCheck(expect *gardener.HTMLNode, got *Stew, errCheck func(msg string, args ...interface{})) {
	if expect.Tag != got.Tag {
		errCheck("@<%d> expected %s, got %s", expect.Pos, expect.Tag, got.Tag)
	}
	if expect.Pos != got.Pos {
		errCheck("@<%s> expected %d, got %d", expect.Tag, expect.Pos, got.Pos)
	}
	if !reflect.DeepEqual(expect.Attrs, got.Attrs) {
		errCheck("@<%d %s> expected %s, got %s", expect.Pos, expect.Tag, expect.Attrs, got.Attrs)
	}
	expectN := len(expect.Children)
	gotN := len(got.Children)
	if expectN != gotN {
		errCheck("@<%d %s> expected %d children, got %d children",
			expect.Pos, expect.Tag, expectN, gotN)
	} else {
		for i, ex := range expect.Children {
			eChild := (*ex).(gardener.HTMLNode)
			gChild := got.Children[i]
			if gChild.Parent != got {
				errCheck("@<%d %s> expected parent to be <%d %s>",
					got.Pos, got.Tag, expect.Pos, expect.Tag)
			} else {
				treeCheck(&eChild, gChild, errCheck)
			}
		}
	}
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

//// ====== Setup ======

// randomly generate a stew and convert it to dom
func setupExpectation() {
	// setup page node
	expectedPage = gardener.GeneratePage(100, nil)
	htmlChild := (*expectedPage.Children[0]).(gardener.HTMLNode)
	headChild := (*htmlChild.Children[0]).(gardener.HTMLNode)
	titleChild := (*headChild.Children[0]).(gardener.HTMLNode)
	titleChild.Attrs[""] = []string{"sample title"}

	// setup page text
	sampleHTML = gardener.ToHTML(expectedPage)

	// setup expectation maps
	expectedTags = make([]struct {
		args []string
		out  []*gardener.HTMLNode
	}, nTagGroup)
	i := 0
	for tag, nodes := range expectedPage.Info.Tags {
		expectedTags[i].args = append(expectedTags[i].args, tag)
		expectedTags[i].out = append(expectedTags[i].out, nodes...)

		i = (i + 1) % nTagGroup
	}

	for attr, nodes := range expectedPage.Info.Attrs {
		for _, node := range nodes {
			if attr == "href" && node.Attrs[attr][0] == "#" {
				continue
			}
			// assert that node.Attrs[attr] is not empty
			expectedAttrs = append(expectedAttrs, struct {
				attr, val string
				out       *gardener.HTMLNode
			}{attr: attr, val: node.Attrs[attr][0], out: node})
		}
	}
}

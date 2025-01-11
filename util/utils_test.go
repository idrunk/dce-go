package util

import (
	"slices"
	"strings"
	"testing"
)

type Tuple struct {
	id  uint8
	pid uint8
	msg string
}

func newTuple(id uint8, pid uint8, msg string) Tuple {
	return Tuple{
		id:  id,
		pid: pid,
		msg: msg,
	}
}

func (t Tuple) Key() uint8 {
	return t.id
}

func (t Tuple) ChildOf(parent any) bool {
	return t.pid == parent.(Tuple).id
}

func (t Tuple) EqualTo(other any) bool {
	return t.id == other.(Tuple).id
}

func TestNewTree(t *testing.T) {
	root := NewTree(newTuple(0, 0, "x"))
	root.Set(newTuple(1, 0, "a"))
	root.Set(newTuple(2, 0, "b"))
	root.SetByPath([]uint8{8}, newTuple(8, 0, "h"))
	root.SetByPath([]uint8{1, 3}, newTuple(3, 1, "c"))
	root.SetByPath([]uint8{1, 4}, newTuple(4, 1, "d"))
	root.SetByPath([]uint8{8, 5}, newTuple(5, 8, "e"))
	root.SetByPath([]uint8{8, 5, 6}, newTuple(6, 5, "f"))
	root.SetByPath([]uint8{8, 5, 6}, newTuple(7, 5, "g"))
	root.SetByPath([]uint8{1, 3, 9}, newTuple(9, 3, "i"))
	root.SetByPath([]uint8{1, 3, 10}, newTuple(10, 3, "j"))
	t.Logf("Root: %v", root)
	root.Traversal(func(tr *Tree[Tuple, uint8]) int {
		t.Logf("Traversal: %v", tr.Element.msg)
		return TreeTraverContinue
	})
}

type Path string

func (p Path) Key() string {
	path := string(p)
	if index := strings.LastIndex(path, "/"); index >= 0 {
		path = path[index+1:]
	}
	return path
}

func (p Path) ChildOf(parent any) bool {
	path := string(p)
	ppath := string(parent.(Path))
	if index := strings.LastIndex(path, "/"); index >= 0 {
		return path[:index] == ppath
	}
	return len(ppath) > 0
}

func (p Path) EqualTo(other any) bool {
	return string(p) == string(other.(Path))
}

func TestTreeBuild(t *testing.T) {
	tree := NewTree(Path(""))
	tree.Build([]Path{
		"hello",
		"hello/world",
		"hello/world/!",
		"hello/rust!",
		"hello/examples/for/rust!",
	}, func(tree *Tree[Path, string], remains []Path) {
		var fills []Path
		for _, remain := range remains {
			paths := strings.Split(string(remain), "/")
			for i := 0; i < len(paths); i++ {
				path := strings.Join(paths[:i+1], "/")
				elem := Path(path)
				if _, ok := tree.ChildByPath(paths[:i+1]); !ok && !slices.Contains(fills, elem) {
					fills = append(fills, elem)
				}
			}
		}
		for _, fill := range fills {
			_, _ = tree.SetByPath(strings.Split(string(fill), "/"), fill)
		}
	})
	tr, ok := tree.ChildByPath([]string{"hello", "world"})
	if !ok {
		t.Fatalf("Failed to ChildByPath")
	}
	tr2, ok := tr.Child("!")
	if !ok {
		t.Fatalf("Failed to Child")
	}
	tr3, _ := tree.Child("hello")
	parents := tr2.ParentsUntil(tr3, false)
	t.Logf("tr: %v", tr)
	t.Logf("tr2: %v", tr2)
	t.Logf("tr3: %v", tr3)
	t.Logf("tree: %v", tree)
	t.Logf("parents: %v", parents)
	tree.Traversal(func(tr *Tree[Path, string]) int {
		t.Logf("Traversal: %v", tr)
		return TreeTraverContinue
	})
}

package util

import (
	"maps"
	"slices"
)

const (
	TreeTraverStop = iota + 1
	TreeTraverBreak
	TreeTraverSkip
	TreeTraverContinue
)

type TreeElement[K comparable] interface {
	Key() K
	ChildOf(parent any) bool
	EqualTo(other any) bool
}

// Tree represents a generic tree structure where each node contains an element of type E.
// The element type E must implement the TreeElement interface, which requires methods for
// retrieving the node's key, determining if it is a child of a given parent, and comparing
// equality with another element. The tree is parameterized by two types:
//   - E: The type of the element stored in each node, which must satisfy the TreeElement[K] interface.
//   - K: The type of the key used to identify nodes, which must be comparable.
//
// The tree structure includes:
//   - Element: The element stored in the current node.
//   - Children: A map of child nodes, indexed by their keys.
//   - Parent: A reference to the parent node, if any.
//
// This structure is designed to support operations such as adding, retrieving, and traversing nodes,
// as well as building and managing hierarchical relationships between elements.
type Tree[E TreeElement[K], K comparable] struct {
	Element  E
	Children map[K]*Tree[E, K]
	Parent   *Tree[E, K]
}

func (t *Tree[E, K]) Set(element E) *Tree[E, K] {
	child := NewTree(element)
	child.Parent = t
	t.Children[element.Key()] = &child
	return &child
}

func (t *Tree[E, K]) SetIfAbsent(element E) *Tree[E, K] {
	if val, ok := t.Children[element.Key()]; ok {
		return val
	}
	return t.Set(element)
}

func (t *Tree[E, K]) SetByPath(path []K, elem E) (*Tree[E, K], error) {
	return t.actualSetByPath(path, elem, true)
}

func (t *Tree[E, K]) SetByPathIfAbsent(path []K, elem E) (*Tree[E, K], error) {
	return t.actualSetByPath(path, elem, false)
}

func (t *Tree[E, K]) actualSetByPath(path []K, elem E, force bool) (*Tree[E, K], error) {
	if len(path) == 0 {
		return nil, Closed0("Cannot get by an empty path")
	}
	parent, ok := t.ChildByPath(path[:len(path)-1])
	if !ok {
		return nil, Closed0("Cannot find Parent by path '%v'", path)
	}
	if force {
		parent.Set(elem)
	} else {
		parent.SetIfAbsent(elem)
	}
	return parent, nil
}

func (t *Tree[E, K]) Child(key K) (*Tree[E, K], bool) {
	value, ok := t.Children[key]
	return value, ok
}

func (t *Tree[E, K]) ChildByPath(path []K) (*Tree[E, K], bool) {
	child := t
	for _, part := range path {
		if c, ok := child.Children[part]; ok {
			child = c
		} else {
			return nil, false
		}
	}
	return child, true
}

func (t *Tree[E, K]) Parents() []*Tree[E, K] {
	return t.ParentsUntil(nil, true)
}

func (t *Tree[E, K]) ParentsUntil(until *Tree[E, K], elderFirst bool) []*Tree[E, K] {
	var parents []*Tree[E, K]
	for parent := t; parent != nil && (until == nil || until.Element.EqualTo(parent.Element)); parent = parent.Parent {
		parents = append(parents, parent)
	}
	if elderFirst {
		slices.Reverse(parents)
	}
	return parents
}

// Traversal performs a breadth-first traversal of the tree starting from the current node.
// The traversal is controlled by the provided callback function, which is called for each node visited.
// The callback function should return one of the following constants to control the traversal behavior:
//   - TreeTraverStop: Stops the traversal entirely.
//   - TreeTraverBreak: Breaks out of the current level of traversal (i.e., stops processing children of the current node).
//   - TreeTraverSkip: Skips the current node's children and continues with the next node at the same level.
//   - TreeTraverContinue: Continues the traversal normally, including the current node's children.
//
// This method is useful for scenarios where you need to traverse the tree and perform actions or checks on each node,
// with the ability to control the flow of the traversal based on the node's content or other conditions.
func (t *Tree[E, K]) Traversal(callback func(*Tree[E, K]) int) {
	nodes := []*Tree[E, K]{t}
Outer:
	for i := 0; i < len(nodes); i++ {
		parent := nodes[i]
		for child := range maps.Values(parent.Children) {
			switch callback(child) {
			case TreeTraverStop:
				break Outer
			case TreeTraverBreak:
				break
			case TreeTraverSkip:
				continue
			case TreeTraverContinue:
				nodes = append(nodes, child)
			}
		}
	}
}

// Build constructs a tree structure from a list of elements. The function takes two parameters:
//   - elements: A slice of elements to be added to the tree. Each element must implement the TreeElement interface.
//   - remainsHandler: An optional callback function that is called with any elements that could not be added to the tree.
//                    This function receives the root of the tree and the remaining elements as arguments.
//
// The function works by iteratively adding elements to the tree based on their hierarchical relationships.
// Elements are added as children of the current node if they satisfy the `ChildOf` condition with respect to the node's element.
// If an element cannot be added to the tree (i.e., it does not satisfy the `ChildOf` condition with any existing node),
// it is passed to the `remainsHandler` callback, if provided.
//
// This method is useful for building a tree from a flat list of elements, where the hierarchical structure is determined
// by the `ChildOf` method of the elements.
func (t *Tree[E, K]) Build(elements []E, remainsHandler func(tree *Tree[E, K], remains []E)) {
	parents := []*Tree[E, K]{t}
	for i := 0; i < len(parents); i++ {
		pa := parents[i]
		var childIndexes []int
		for j := len(elements) - 1; j >= 0; j-- {
			if elements[j].ChildOf(pa.Element) {
				childIndexes = append(childIndexes, j)
			}
		}
		for _, childIndex := range childIndexes {
			elem := elements[childIndex]
			parents = append(parents, pa.Set(elem))
			elements = slices.Delete(elements, childIndex, childIndex+1)
		}
	}
	if remainsHandler != nil {
		remainsHandler(t, elements)
	}
}

// NewTree init a tree instance use the given element
func NewTree[E TreeElement[K], K comparable](elem E) Tree[E, K] {
	return Tree[E, K]{
		Element:  elem,
		Children: make(map[K]*Tree[E, K]),
		Parent:   nil,
	}
}

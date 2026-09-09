package dept

import (
	"sort"
	"strings"

	"github.com/google/uuid"
)

// ChildPath computes the path of a node placed under parentPath ("" = root).
// Paths always look like "/id1/id2/" and include the node itself.
func ChildPath(parentPath string, id uuid.UUID) string {
	if parentPath == "" {
		return "/" + id.String() + "/"
	}
	return parentPath + id.String() + "/"
}

// InSubtree reports whether the node with path candidate is self or one of its
// descendants — exactly the places a node must not be moved to.
func InSubtree(candidate, selfPath string) bool {
	return strings.HasPrefix(candidate, selfPath)
}

// RebasePath swaps the oldPrefix of path for newPrefix (what the subtree
// UPDATE does in SQL; kept here so the rule is unit-testable).
func RebasePath(path, oldPrefix, newPrefix string) string {
	if !strings.HasPrefix(path, oldPrefix) {
		return path
	}
	return newPrefix + path[len(oldPrefix):]
}

// BuildTree nests a flat list into roots → children, every level ordered by
// (sort, name). A node whose parent is missing from the list is treated as a
// root so the tree never silently loses departments.
func BuildTree(depts []Dept) []*Node {
	nodes := make(map[uuid.UUID]*Node, len(depts))
	order := make([]*Node, 0, len(depts))
	for i := range depts {
		n := &Node{Dept: depts[i], Children: []*Node{}}
		nodes[n.ID] = n
		order = append(order, n)
	}
	roots := []*Node{}
	for _, n := range order {
		if n.ParentID != nil {
			if p, ok := nodes[*n.ParentID]; ok {
				p.Children = append(p.Children, n)
				continue
			}
		}
		roots = append(roots, n)
	}
	sortNodes(roots)
	for _, n := range order {
		sortNodes(n.Children)
	}
	return roots
}

func sortNodes(ns []*Node) {
	sort.SliceStable(ns, func(i, j int) bool {
		if ns[i].Sort != ns[j].Sort {
			return ns[i].Sort < ns[j].Sort
		}
		return ns[i].Name < ns[j].Name
	})
}

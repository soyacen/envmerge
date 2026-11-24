package resolver

import "path/filepath"

// Dependencies returns the absolute paths of every document reachable from root
// (excluding the root itself), in deterministic first-visit order.
func Dependencies(root *ResolvedDocument) []string {
	seen := map[string]bool{root.ID: true}
	var out []string
	var walk func(*ResolvedDocument)
	walk = func(node *ResolvedDocument) {
		for _, child := range node.Includes {
			if !seen[child.ID] {
				seen[child.ID] = true
				out = append(out, child.AbsPath)
			}
			walk(child)
		}
	}
	walk(root)
	return out
}

// TreeNode is a line of the visual dependency tree produced by the CLI.
type TreeNode struct {
	Name     string
	Children []*TreeNode
}

// Tree builds a renderable tree. Because a shared dependency can appear under
// several parents, the tree shows it each time it is referenced (mirroring the
// resolution order) but stops descending into an already-expanded branch to
// avoid infinite depth on diamonds.
func Tree(root *ResolvedDocument) *TreeNode {
	expanded := map[string]bool{}
	return buildTree(root, expanded)
}

func buildTree(node *ResolvedDocument, expanded map[string]bool) *TreeNode {
	name := filepath.Base(node.AbsPath)
	t := &TreeNode{Name: name}
	if expanded[node.ID] {
		return t
	}
	expanded[node.ID] = true
	for _, child := range node.Includes {
		t.Children = append(t.Children, buildTree(child, expanded))
	}
	return t
}

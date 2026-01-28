package plan

import "fmt"

// WalkTree recursively walks the plan tree, calling visitor for each node
func WalkTree(node Node, visitor func(Node) error) error {
	if node == nil {
		return nil
	}
	
	// Visit current node
	if err := visitor(node); err != nil {
		return err
	}
	
	// Recursively visit children
	for _, child := range node.Children() {
		if err := WalkTree(child, visitor); err != nil {
			return err
		}
	}
	
	return nil
}

// PrintTree prints the plan tree with indentation
func PrintTree(node Node) string {
	result := ""
	printTreeHelper(node, 0, &result)
	return result
}

func printTreeHelper(node Node, depth int, result *string) {
	if node == nil {
		return
	}
	
	// Print current node with indentation
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	*result += fmt.Sprintf("%s%s\n", indent, node.NodeType())
	
	// Recursively print children
	for _, child := range node.Children() {
		printTreeHelper(child, depth+1, result)
	}
}

// CountNodes counts the total number of nodes in the tree
func CountNodes(node Node) int {
	if node == nil {
		return 0
	}
	
	count := 1 // Count current node
	for _, child := range node.Children() {
		count += CountNodes(child)
	}
	
	return count
}

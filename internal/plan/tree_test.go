package plan

import (
	"strings"
	"testing"

	"github.com/leengari/mini-rdbms/internal/domain/transaction"
)

// TestTreeStructure verifies that nodes form a tree
func TestTreeStructure(t *testing.T) {
	tx := transaction.NewTransaction()
	
	// Create a simple tree: SelectNode -> JoinNode -> (ScanNode, ScanNode)
	leftScan := &ScanNode{
		TableName:   "users",
		Transaction: tx,
	}
	
	rightScan := &ScanNode{
		TableName:   "orders",
		Transaction: tx,
	}
	
	joinNode := NewJoinNode(leftScan, rightScan, 0, "id", "user_id")
	
	selectNode := &SelectNode{
		TableName:   "users",
		Transaction: tx,
	}
	selectNode.AddChild(joinNode)
	
	// Verify tree structure
	if len(selectNode.Children()) != 1 {
		t.Errorf("SelectNode should have 1 child, got %d", len(selectNode.Children()))
	}
	
	if len(joinNode.Children()) != 2 {
		t.Errorf("JoinNode should have 2 children, got %d", len(joinNode.Children()))
	}
	
	if len(leftScan.Children()) != 0 {
		t.Errorf("ScanNode should have 0 children, got %d", len(leftScan.Children()))
	}
}

// TestMetadata verifies metadata attachment
func TestMetadata(t *testing.T) {
	node := &ScanNode{TableName: "users"}
	
	// Metadata should never be nil
	if node.Metadata() == nil {
		t.Error("Metadata() should never return nil")
	}
	
	// Attach metadata
	node.Metadata()["test_key"] = "test_value"
	node.Metadata()["estimated_rows"] = 1000
	
	// Read metadata
	if val, ok := node.Metadata()["test_key"].(string); !ok || val != "test_value" {
		t.Errorf("Expected test_key='test_value', got %v", node.Metadata()["test_key"])
	}
	
	if val, ok := node.Metadata()["estimated_rows"].(int); !ok || val != 1000 {
		t.Errorf("Expected estimated_rows=1000, got %v", node.Metadata()["estimated_rows"])
	}
}

// TestWalkTree verifies tree walking
func TestWalkTree(t *testing.T) {
	tx := transaction.NewTransaction()
	
	// Create tree
	leftScan := &ScanNode{TableName: "users", Transaction: tx}
	rightScan := &ScanNode{TableName: "orders", Transaction: tx}
	joinNode := NewJoinNode(leftScan, rightScan, 0, "id", "user_id")
	selectNode := &SelectNode{TableName: "users", Transaction: tx}
	selectNode.AddChild(joinNode)
	
	// Walk tree and count nodes
	nodeCount := 0
	err := WalkTree(selectNode, func(n Node) error {
		nodeCount++
		return nil
	})
	
	if err != nil {
		t.Errorf("WalkTree failed: %v", err)
	}
	
	// Should visit: SelectNode, JoinNode, ScanNode (left), ScanNode (right) = 4 nodes
	if nodeCount != 4 {
		t.Errorf("Expected to visit 4 nodes, visited %d", nodeCount)
	}
}

// TestPrintTree verifies tree printing
func TestPrintTree(t *testing.T) {
	tx := transaction.NewTransaction()
	
	// Create tree
	leftScan := &ScanNode{TableName: "users", Transaction: tx}
	rightScan := &ScanNode{TableName: "orders", Transaction: tx}
	joinNode := NewJoinNode(leftScan, rightScan, 0, "id", "user_id")
	selectNode := &SelectNode{TableName: "users", Transaction: tx}
	selectNode.AddChild(joinNode)
	
	// Print tree
	output := PrintTree(selectNode)
	
	// Verify output contains expected node types
	if !strings.Contains(output, "SELECT") {
		t.Error("Tree output should contain SELECT")
	}
	if !strings.Contains(output, "JOIN") {
		t.Error("Tree output should contain JOIN")
	}
	if !strings.Contains(output, "SCAN") {
		t.Error("Tree output should contain SCAN")
	}
}

// TestCountNodes verifies node counting
func TestCountNodes(t *testing.T) {
	tx := transaction.NewTransaction()
	
	// Create tree
	leftScan := &ScanNode{TableName: "users", Transaction: tx}
	rightScan := &ScanNode{TableName: "orders", Transaction: tx}
	joinNode := NewJoinNode(leftScan, rightScan, 0, "id", "user_id")
	selectNode := &SelectNode{TableName: "users", Transaction: tx}
	selectNode.AddChild(joinNode)
	
	// Count nodes
	count := CountNodes(selectNode)
	
	// Should count: SelectNode, JoinNode, ScanNode (left), ScanNode (right) = 4 nodes
	if count != 4 {
		t.Errorf("Expected 4 nodes, got %d", count)
	}
}

// TestNodeType verifies NodeType method
func TestNodeType(t *testing.T) {
	tests := []struct {
		node     Node
		expected string
	}{
		{&ScanNode{}, "SCAN"},
		{&JoinNode{}, "JOIN"},
		{&SelectNode{}, "SELECT"},
		{&InsertNode{}, "INSERT"},
		{&UpdateNode{}, "UPDATE"},
		{&DeleteNode{}, "DELETE"},
	}
	
	for _, tt := range tests {
		if tt.node.NodeType() != tt.expected {
			t.Errorf("Expected NodeType=%s, got %s", tt.expected, tt.node.NodeType())
		}
	}
}

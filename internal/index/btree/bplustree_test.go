package btree

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

// ---------------------------------------------------------------------
// Basic Insert / Search
// ---------------------------------------------------------------------

func TestInsertAndSearch_Int64(t *testing.T) {
	tree := New(3) // small degree for easier debugging
	keys := []int64{10, 20, 5, 15, 25, 30, 1, 7}

	for i, k := range keys {
		if err := tree.Insert(k, i); err != nil {
			t.Fatalf("Insert(%d): %v", k, err)
		}
	}

	if tree.Size() != len(keys) {
		t.Fatalf("Size = %d, want %d", tree.Size(), len(keys))
	}

	for i, k := range keys {
		pos, found := tree.Search(k)
		if !found {
			t.Errorf("Search(%d): not found", k)
		}
		if pos != i {
			t.Errorf("Search(%d): pos = %d, want %d", k, pos, i)
		}
	}

	// Search for non-existent key.
	_, found := tree.Search(int64(999))
	if found {
		t.Error("Search(999): should not be found")
	}
}

func TestInsertAndSearch_String(t *testing.T) {
	tree := New(3)
	keys := []string{"banana", "apple", "cherry", "date", "elderberry"}

	for i, k := range keys {
		if err := tree.Insert(k, i); err != nil {
			t.Fatalf("Insert(%s): %v", k, err)
		}
	}

	for i, k := range keys {
		pos, found := tree.Search(k)
		if !found {
			t.Errorf("Search(%s): not found", k)
		}
		if pos != i {
			t.Errorf("Search(%s): pos = %d, want %d", k, pos, i)
		}
	}
}

func TestInsertAndSearch_Float64(t *testing.T) {
	tree := New(3)
	keys := []float64{3.14, 2.71, 1.41, 9.81, 6.02}

	for i, k := range keys {
		if err := tree.Insert(k, i); err != nil {
			t.Fatalf("Insert(%f): %v", k, err)
		}
	}

	for i, k := range keys {
		pos, found := tree.Search(k)
		if !found {
			t.Errorf("Search(%f): not found", k)
		}
		if pos != i {
			t.Errorf("Search(%f): pos = %d, want %d", k, pos, i)
		}
	}
}

// ---------------------------------------------------------------------
// Cross-type numeric comparison (critical for JSON deserialization)
// ---------------------------------------------------------------------

func TestCrossTypeNumericSearch(t *testing.T) {
	tree := New(3)

	// Insert as int64 (how auto-increment PKs are stored).
	if err := tree.Insert(int64(42), 0); err != nil {
		t.Fatal(err)
	}

	// Search as int (Go default integer literal type).
	pos, found := tree.Search(int(42))
	if !found {
		t.Fatal("Search(int(42)): not found when inserted as int64")
	}
	if pos != 0 {
		t.Fatalf("pos = %d, want 0", pos)
	}

	// Search as float64 (how JSON numbers deserialize).
	pos, found = tree.Search(float64(42))
	if !found {
		t.Fatal("Search(float64(42)): not found when inserted as int64")
	}
	if pos != 0 {
		t.Fatalf("pos = %d, want 0", pos)
	}
}

// ---------------------------------------------------------------------
// Duplicate key rejection
// ---------------------------------------------------------------------

func TestDuplicateKeyRejected(t *testing.T) {
	tree := New(3)
	if err := tree.Insert(int64(1), 0); err != nil {
		t.Fatal(err)
	}
	err := tree.Insert(int64(1), 1)
	if err != ErrDuplicateKey {
		t.Fatalf("expected ErrDuplicateKey, got %v", err)
	}
	if tree.Size() != 1 {
		t.Fatalf("Size = %d after duplicate, want 1", tree.Size())
	}
}

// ---------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------

func TestDelete_Basic(t *testing.T) {
	tree := New(3)
	keys := []int64{10, 20, 5, 15, 25, 30, 1, 7}
	for i, k := range keys {
		tree.Insert(k, i)
	}

	// Delete a leaf key.
	if err := tree.Delete(int64(7)); err != nil {
		t.Fatalf("Delete(7): %v", err)
	}
	if tree.Size() != len(keys)-1 {
		t.Fatalf("Size = %d, want %d", tree.Size(), len(keys)-1)
	}
	if _, found := tree.Search(int64(7)); found {
		t.Error("Search(7): should not be found after delete")
	}

	// Other keys should still be found.
	for i, k := range keys {
		if k == 7 {
			continue
		}
		pos, found := tree.Search(k)
		if !found {
			t.Errorf("Search(%d): not found after deleting 7", k)
		}
		if pos != i {
			t.Errorf("Search(%d): pos = %d, want %d", k, pos, i)
		}
	}
}

func TestDelete_NotFound(t *testing.T) {
	tree := New(3)
	tree.Insert(int64(1), 0)
	err := tree.Delete(int64(999))
	if err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestDelete_AllKeys(t *testing.T) {
	tree := New(3)
	n := 50
	keys := make([]int64, n)
	for i := 0; i < n; i++ {
		keys[i] = int64(i)
		tree.Insert(keys[i], i)
	}

	// Delete all keys in random order.
	rng := rand.New(rand.NewSource(42))
	perm := rng.Perm(n)
	for _, idx := range perm {
		k := keys[idx]
		if err := tree.Delete(k); err != nil {
			t.Fatalf("Delete(%d): %v", k, err)
		}
		// Verify the deleted key is gone.
		if _, found := tree.Search(k); found {
			t.Fatalf("Search(%d): still found after delete", k)
		}
	}

	if tree.Size() != 0 {
		t.Fatalf("Size = %d after deleting all, want 0", tree.Size())
	}
}

func TestDelete_WithMergesAndBorrows(t *testing.T) {
	// Use degree 2 (min keys=1, max keys=3) to force merges/borrows frequently.
	tree := New(2)
	n := 30
	for i := 0; i < n; i++ {
		tree.Insert(int64(i), i)
	}

	// Delete keys in a pattern that triggers both borrow-left and borrow-right.
	deleteOrder := []int64{0, 2, 4, 6, 8, 1, 3, 5, 7, 9, 15, 14, 13, 12, 11, 10}
	for _, k := range deleteOrder {
		if err := tree.Delete(k); err != nil {
			t.Fatalf("Delete(%d): %v", k, err)
		}
	}

	// Remaining keys should still be accessible.
	remaining := make(map[int64]bool)
	for i := 0; i < n; i++ {
		remaining[int64(i)] = true
	}
	for _, k := range deleteOrder {
		delete(remaining, k)
	}
	for k := range remaining {
		if _, found := tree.Search(k); !found {
			t.Errorf("Search(%d): not found after partial deletes", k)
		}
	}
}

// ---------------------------------------------------------------------
// RangeScan
// ---------------------------------------------------------------------

func TestRangeScan(t *testing.T) {
	tree := New(3)
	// Insert keys 0..19
	for i := 0; i < 20; i++ {
		tree.Insert(int64(i), i)
	}

	// Range [5, 14]
	result := tree.RangeScan(int64(5), int64(14))
	expected := []int{5, 6, 7, 8, 9, 10, 11, 12, 13, 14}
	if len(result) != len(expected) {
		t.Fatalf("RangeScan(5,14): got %d results, want %d", len(result), len(expected))
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("RangeScan(5,14)[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestRangeScan_EmptyResult(t *testing.T) {
	tree := New(3)
	for i := 0; i < 10; i++ {
		tree.Insert(int64(i), i)
	}
	result := tree.RangeScan(int64(100), int64(200))
	if len(result) != 0 {
		t.Fatalf("RangeScan(100,200): expected empty, got %v", result)
	}
}

func TestRangeScan_SingleKey(t *testing.T) {
	tree := New(3)
	for i := 0; i < 10; i++ {
		tree.Insert(int64(i), i)
	}
	result := tree.RangeScan(int64(5), int64(5))
	if len(result) != 1 || result[0] != 5 {
		t.Fatalf("RangeScan(5,5) = %v, want [5]", result)
	}
}

func TestRangeScan_Strings(t *testing.T) {
	tree := New(3)
	words := []string{"apple", "banana", "cherry", "date", "elderberry", "fig", "grape"}
	for i, w := range words {
		tree.Insert(w, i)
	}

	// Range ["cherry", "fig"] — sorted: apple, banana, cherry, date, elderberry, fig, grape
	result := tree.RangeScan("cherry", "fig")
	// Expected positions for cherry(2), date(3), elderberry(4), fig(5)
	expected := []int{2, 3, 4, 5}
	if len(result) != len(expected) {
		t.Fatalf("RangeScan(cherry, fig): got %d results, want %d: %v", len(result), len(expected), result)
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("RangeScan[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

// ---------------------------------------------------------------------
// All (full scan via linked list)
// ---------------------------------------------------------------------

func TestAll_Ordered(t *testing.T) {
	tree := New(3)
	// Insert in random order.
	keys := []int64{50, 10, 40, 20, 30}
	for i, k := range keys {
		tree.Insert(k, i)
	}

	all := tree.All()
	if len(all) != len(keys) {
		t.Fatalf("All(): got %d entries, want %d", len(all), len(keys))
	}

	// Values should be in key order: 10→1, 20→3, 30→4, 40→2, 50→0
	expected := []int{1, 3, 4, 2, 0}
	for i, v := range all {
		if v != expected[i] {
			t.Errorf("All()[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

// ---------------------------------------------------------------------
// Clear
// ---------------------------------------------------------------------

func TestClear(t *testing.T) {
	tree := New(3)
	for i := 0; i < 20; i++ {
		tree.Insert(int64(i), i)
	}
	tree.Clear()
	if tree.Size() != 0 {
		t.Fatalf("Size after Clear = %d, want 0", tree.Size())
	}
	if _, found := tree.Search(int64(0)); found {
		t.Error("Search(0) found after Clear")
	}
	// Should be able to insert again.
	if err := tree.Insert(int64(42), 0); err != nil {
		t.Fatalf("Insert after Clear: %v", err)
	}
}

// ---------------------------------------------------------------------
// UpdatePos
// ---------------------------------------------------------------------

func TestUpdatePos(t *testing.T) {
	tree := New(3)
	tree.Insert(int64(10), 0)
	tree.Insert(int64(20), 1)

	if err := tree.UpdatePos(int64(10), 99); err != nil {
		t.Fatal(err)
	}
	pos, _ := tree.Search(int64(10))
	if pos != 99 {
		t.Fatalf("after UpdatePos: pos = %d, want 99", pos)
	}

	err := tree.UpdatePos(int64(999), 0)
	if err != ErrKeyNotFound {
		t.Fatalf("UpdatePos(999): expected ErrKeyNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------
// Empty tree
// ---------------------------------------------------------------------

func TestEmptyTree(t *testing.T) {
	tree := New(3)
	if tree.Size() != 0 {
		t.Fatal("new tree should be empty")
	}
	if _, found := tree.Search(int64(1)); found {
		t.Fatal("search in empty tree should return false")
	}
	result := tree.All()
	if len(result) != 0 {
		t.Fatal("All() on empty tree should return empty")
	}
	result = tree.RangeScan(int64(0), int64(100))
	if len(result) != 0 {
		t.Fatal("RangeScan on empty tree should return empty")
	}
}

// ---------------------------------------------------------------------
// Single element
// ---------------------------------------------------------------------

func TestSingleElement(t *testing.T) {
	tree := New(3)
	tree.Insert(int64(42), 7)

	pos, found := tree.Search(int64(42))
	if !found || pos != 7 {
		t.Fatalf("Search(42) = (%d, %v), want (7, true)", pos, found)
	}

	all := tree.All()
	if len(all) != 1 || all[0] != 7 {
		t.Fatalf("All() = %v, want [7]", all)
	}

	tree.Delete(int64(42))
	if tree.Size() != 0 {
		t.Fatal("size should be 0 after deleting only element")
	}
}

// ---------------------------------------------------------------------
// Large-scale: 10k keys, verify ordering and tree depth
// ---------------------------------------------------------------------

func TestLargeScale_10k(t *testing.T) {
	tree := New(DefaultDegree) // degree 32 as in production
	n := 10000

	// Insert in random order.
	rng := rand.New(rand.NewSource(12345))
	keys := rng.Perm(n)

	for _, k := range keys {
		if err := tree.Insert(int64(k), k); err != nil {
			t.Fatalf("Insert(%d): %v", k, err)
		}
	}

	if tree.Size() != n {
		t.Fatalf("Size = %d, want %d", tree.Size(), n)
	}

	// Verify every key is searchable.
	for _, k := range keys {
		pos, found := tree.Search(int64(k))
		if !found {
			t.Fatalf("Search(%d): not found", k)
		}
		if pos != k {
			t.Fatalf("Search(%d): pos = %d, want %d", k, pos, k)
		}
	}

	// Verify All() returns keys in sorted order.
	all := tree.All()
	if len(all) != n {
		t.Fatalf("All() length = %d, want %d", len(all), n)
	}
	for i := 0; i < n; i++ {
		if all[i] != i {
			t.Fatalf("All()[%d] = %d, want %d (key-order traversal broken)", i, all[i], i)
		}
	}

	// Verify tree depth is reasonable (should be 3-4 for 10k keys at degree 32).
	depth := treeDepth(tree.root)
	if depth > 4 {
		t.Errorf("tree depth = %d for %d keys at degree %d, expected <= 4", depth, n, DefaultDegree)
	}
	t.Logf("Tree depth for %d keys at degree %d: %d", n, DefaultDegree, depth)
}

func TestLargeScale_InsertDeleteMix(t *testing.T) {
	tree := New(DefaultDegree)
	n := 5000

	// Insert all keys.
	for i := 0; i < n; i++ {
		tree.Insert(int64(i), i)
	}

	// Delete half (even numbers).
	for i := 0; i < n; i += 2 {
		if err := tree.Delete(int64(i)); err != nil {
			t.Fatalf("Delete(%d): %v", i, err)
		}
	}

	if tree.Size() != n/2 {
		t.Fatalf("Size = %d, want %d", tree.Size(), n/2)
	}

	// Verify remaining odd keys.
	for i := 1; i < n; i += 2 {
		pos, found := tree.Search(int64(i))
		if !found {
			t.Fatalf("Search(%d): not found", i)
		}
		if pos != i {
			t.Fatalf("Search(%d): pos = %d, want %d", i, pos, i)
		}
	}

	// Verify deleted even keys are gone.
	for i := 0; i < n; i += 2 {
		if _, found := tree.Search(int64(i)); found {
			t.Fatalf("Search(%d): should not be found", i)
		}
	}

	// Verify All() returns only odd keys in order.
	all := tree.All()
	if len(all) != n/2 {
		t.Fatalf("All() length = %d, want %d", len(all), n/2)
	}
	for idx, v := range all {
		expected := idx*2 + 1
		if v != expected {
			t.Fatalf("All()[%d] = %d, want %d", idx, v, expected)
		}
	}
}

// ---------------------------------------------------------------------
// RangeScan correctness at scale
// ---------------------------------------------------------------------

func TestRangeScan_LargeScale(t *testing.T) {
	tree := New(DefaultDegree)
	n := 1000
	for i := 0; i < n; i++ {
		tree.Insert(int64(i), i)
	}

	lo, hi := int64(200), int64(400)
	result := tree.RangeScan(lo, hi)
	expected := int(hi - lo + 1)
	if len(result) != expected {
		t.Fatalf("RangeScan(%d,%d): got %d results, want %d", lo, hi, len(result), expected)
	}
	for i, v := range result {
		if v != int(lo)+i {
			t.Errorf("RangeScan[%d] = %d, want %d", i, v, int(lo)+i)
		}
	}
}

// ---------------------------------------------------------------------
// Linked list integrity
// ---------------------------------------------------------------------

func TestLeafLinkedList_Integrity(t *testing.T) {
	tree := New(3)
	n := 50
	for i := 0; i < n; i++ {
		tree.Insert(int64(i), i)
	}

	// Walk forward from first.
	count := 0
	var lastKey interface{}
	node := tree.first
	for node != nil {
		for _, k := range node.keys {
			if lastKey != nil && compareKeys(k, lastKey) <= 0 {
				t.Fatalf("linked list order broken: %v <= %v", k, lastKey)
			}
			lastKey = k
			count++
		}
		node = node.next
	}
	if count != n {
		t.Fatalf("forward walk counted %d entries, want %d", count, n)
	}

	// Walk backward from last leaf (find it via forward walk).
	last := tree.first
	for last.next != nil {
		last = last.next
	}
	count = 0
	lastKey = nil
	node = last
	for node != nil {
		for i := len(node.keys) - 1; i >= 0; i-- {
			k := node.keys[i]
			if lastKey != nil && compareKeys(k, lastKey) >= 0 {
				t.Fatalf("backward linked list order broken: %v >= %v", k, lastKey)
			}
			lastKey = k
			count++
		}
		node = node.prev
	}
	if count != n {
		t.Fatalf("backward walk counted %d entries, want %d", count, n)
	}
}

func TestLeafLinkedList_AfterDeletes(t *testing.T) {
	tree := New(3)
	n := 50
	for i := 0; i < n; i++ {
		tree.Insert(int64(i), i)
	}

	// Delete every other key.
	for i := 0; i < n; i += 2 {
		tree.Delete(int64(i))
	}

	// Verify forward walk yields n/2 keys in order.
	count := 0
	var lastKey interface{}
	node := tree.first
	for node != nil {
		for _, k := range node.keys {
			if lastKey != nil && compareKeys(k, lastKey) <= 0 {
				t.Fatalf("linked list order broken after deletes: %v <= %v", k, lastKey)
			}
			lastKey = k
			count++
		}
		node = node.next
	}
	if count != n/2 {
		t.Fatalf("forward walk after deletes counted %d, want %d", count, n/2)
	}
}

// ---------------------------------------------------------------------
// compareKeys unit tests
// ---------------------------------------------------------------------

func TestCompareKeys(t *testing.T) {
	tests := []struct {
		a, b interface{}
		want int
	}{
		{int64(1), int64(2), -1},
		{int64(2), int64(1), 1},
		{int64(1), int64(1), 0},
		{int64(1), int(1), 0},
		{int(1), int64(1), 0},
		{int64(1), float64(1.0), 0},
		{float64(1.0), int64(1), 0},
		{float64(1.5), float64(2.5), -1},
		{"abc", "def", -1},
		{"def", "abc", 1},
		{"abc", "abc", 0},
	}
	for _, tt := range tests {
		got := compareKeys(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("compareKeys(%v, %v) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------
// Stress: random insert/delete/search cycles
// ---------------------------------------------------------------------

func TestStress_RandomOps(t *testing.T) {
	tree := New(4)
	rng := rand.New(rand.NewSource(99))
	present := make(map[int64]int) // key → pos

	for i := 0; i < 5000; i++ {
		op := rng.Intn(3)
		key := int64(rng.Intn(500))

		switch op {
		case 0: // insert
			if _, exists := present[key]; exists {
				continue
			}
			pos := rng.Intn(10000)
			if err := tree.Insert(key, pos); err != nil {
				t.Fatalf("iteration %d: Insert(%d): %v", i, key, err)
			}
			present[key] = pos

		case 1: // delete
			if _, exists := present[key]; !exists {
				continue
			}
			if err := tree.Delete(key); err != nil {
				t.Fatalf("iteration %d: Delete(%d): %v", i, key, err)
			}
			delete(present, key)

		case 2: // search
			pos, found := tree.Search(key)
			_, shouldExist := present[key]
			if found != shouldExist {
				t.Fatalf("iteration %d: Search(%d): found=%v, shouldExist=%v", i, key, found, shouldExist)
			}
			if found && pos != present[key] {
				t.Fatalf("iteration %d: Search(%d): pos=%d, want %d", i, key, pos, present[key])
			}
		}
	}

	// Final consistency check.
	if tree.Size() != len(present) {
		t.Fatalf("final Size = %d, want %d", tree.Size(), len(present))
	}

	// Verify All() is sorted and contains exactly the right keys.
	all := tree.All()
	if len(all) != len(present) {
		t.Fatalf("All() length = %d, want %d", len(all), len(present))
	}

	// Collect sorted keys from map.
	sortedKeys := make([]int64, 0, len(present))
	for k := range present {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Slice(sortedKeys, func(i, j int) bool { return sortedKeys[i] < sortedKeys[j] })

	for i, k := range sortedKeys {
		if all[i] != present[k] {
			t.Fatalf("All()[%d] = %d, want %d (key=%d)", i, all[i], present[k], k)
		}
	}
}

// ---------------------------------------------------------------------
// Degree edge case
// ---------------------------------------------------------------------

func TestDegree_MinClampedTo2(t *testing.T) {
	tree := New(0) // should be clamped to 2
	tree.Insert(int64(1), 0)
	tree.Insert(int64(2), 1)
	tree.Insert(int64(3), 2)
	if tree.Size() != 3 {
		t.Fatalf("Size = %d, want 3", tree.Size())
	}
}

// ---------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------

func treeDepth(n *node) int {
	if n == nil {
		return 0
	}
	if n.leaf {
		return 1
	}
	return 1 + treeDepth(n.children[0])
}

// ---------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------

func BenchmarkInsert(b *testing.B) {
	for _, degree := range []int{4, 16, 32} {
		b.Run(fmt.Sprintf("degree=%d", degree), func(b *testing.B) {
			tree := New(degree)
			for i := 0; i < b.N; i++ {
				tree.Insert(int64(i), i)
			}
		})
	}
}

func BenchmarkSearch(b *testing.B) {
	tree := New(DefaultDegree)
	n := 100000
	for i := 0; i < n; i++ {
		tree.Insert(int64(i), i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Search(int64(i % n))
	}
}

// ---------------------------------------------------------------------
// RangeFrom / RangeTo
// ---------------------------------------------------------------------

func TestRangeFrom(t *testing.T) {
	tree := New(3)
	// Insert keys 0..19
	for i := 0; i < 20; i++ {
		tree.Insert(int64(i), i)
	}

	// RangeFrom(12) => [12..19]
	result := tree.RangeFrom(int64(12))
	expected := []int{12, 13, 14, 15, 16, 17, 18, 19}
	if len(result) != len(expected) {
		t.Fatalf("RangeFrom(12): got %v (len %d), want %v", result, len(result), expected)
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("RangeFrom(12)[%d] = %d, want %d", i, v, expected[i])
		}
	}

	// RangeFrom(25) => empty
	resultEmpty := tree.RangeFrom(int64(25))
	if len(resultEmpty) != 0 {
		t.Fatalf("RangeFrom(25): expected empty, got %v", resultEmpty)
	}

	// RangeFrom(-5) => [0..19]
	resultAll := tree.RangeFrom(int64(-5))
	if len(resultAll) != 20 {
		t.Fatalf("RangeFrom(-5): expected 20 results, got %d", len(resultAll))
	}
}

func TestRangeTo(t *testing.T) {
	tree := New(3)
	// Insert keys 0..19
	for i := 0; i < 20; i++ {
		tree.Insert(int64(i), i)
	}

	// RangeTo(7) => [0..7]
	result := tree.RangeTo(int64(7))
	expected := []int{0, 1, 2, 3, 4, 5, 6, 7}
	if len(result) != len(expected) {
		t.Fatalf("RangeTo(7): got %v (len %d), want %v", result, len(result), expected)
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("RangeTo(7)[%d] = %d, want %d", i, v, expected[i])
		}
	}

	// RangeTo(-5) => empty
	resultEmpty := tree.RangeTo(int64(-5))
	if len(resultEmpty) != 0 {
		t.Fatalf("RangeTo(-5): expected empty, got %v", resultEmpty)
	}

	// RangeTo(25) => [0..19]
	resultAll := tree.RangeTo(int64(25))
	if len(resultAll) != 20 {
		t.Fatalf("RangeTo(25): expected 20 results, got %d", len(resultAll))
	}
}


package btree

import (
	"fmt"
)

// DefaultDegree is the minimum degree (t) of the B+Tree.
// Each internal node holds up to 2t-1 keys and 2t children.
// Each leaf node holds up to 2t-1 key→pos pairs.
// With t=32: max 63 keys per node, 64 children per internal node.
const DefaultDegree = 32

var ErrDuplicateKey = fmt.Errorf("btree: duplicate key")

var ErrKeyNotFound = fmt.Errorf("btree: key not found")

// node is a B+Tree node. Internal and leaf nodes share the same struct
// to keep allocation simple. Leaf nodes use values/next/prev; internal
// nodes use children.
type node struct {
	keys     []interface{} // routing keys (internal) or data keys (leaf)
	values   []int64         // row positions — only used in leaf nodes
	children []*node       // child pointers — only used in internal nodes
	next     *node         // next leaf — only used in leaf nodes
	prev     *node         // prev leaf — only used in leaf nodes
	leaf     bool
}

// BPlusTree is an ordered map from a comparable key to an int (row position
// in the dense row array). Internal nodes store only routing keys (no values).
// Leaf nodes store key→pos pairs and are linked in a doubly-linked list for
// efficient range scans.
type BPlusTree struct {
	root   *node
	degree int // minimum degree t: nodes hold [t-1, 2t-1] keys
	size   int // total number of entries
	first  *node // pointer to leftmost leaf for full scans
}

// New creates a new BPlusTree with the given minimum degree.
// Use DefaultDegree (32) for production.
func New(degree int) *BPlusTree {
	if degree < 2 {
		degree = 2
	}
	leaf := &node{
		keys:   make([]interface{}, 0, 2*degree-1),
		values: make([]int64, 0, 2*degree-1),
		leaf:   true,
	}
	return &BPlusTree{
		root:   leaf,
		degree: degree,
		size:   0,
		first:  leaf,
	}
}

// Size returns the number of entries in the tree.
func (t *BPlusTree) Size() int {
	return t.size
}

// Clear removes all entries and resets the tree to an empty state.
func (t *BPlusTree) Clear() {
	leaf := &node{
		keys:   make([]interface{}, 0, 2*t.degree-1),
		values: make([]int64, 0, 2*t.degree-1),
		leaf:   true,
	}
	t.root = leaf
	t.first = leaf
	t.size = 0
}

// Key represents a strictly typed B+Tree key.
type Key interface {
	Compare(other Key) int
}

type IntKey int64

func (k IntKey) Compare(other Key) int {
	switch o := other.(type) {
	case IntKey:
		if k < o {
			return -1
		}
		if k > o {
			return 1
		}
		return 0
	case FloatKey:
		fk := float64(k)
		fo := float64(o)
		if fk < fo {
			return -1
		}
		if fk > fo {
			return 1
		}
		return 0
	default:
		panic(fmt.Sprintf("btree: incompatible key type for comparison: %T and %T", k, other))
	}
}

type FloatKey float64

func (k FloatKey) Compare(other Key) int {
	switch o := other.(type) {
	case FloatKey:
		if k < o {
			return -1
		}
		if k > o {
			return 1
		}
		return 0
	case IntKey:
		fk := float64(k)
		fo := float64(o)
		if fk < fo {
			return -1
		}
		if fk > fo {
			return 1
		}
		return 0
	default:
		panic(fmt.Sprintf("btree: incompatible key type for comparison: %T and %T", k, other))
	}
}

type StringKey string

func (k StringKey) Compare(other Key) int {
	o, ok := other.(StringKey)
	if !ok {
		panic(fmt.Sprintf("btree: incompatible key type for comparison: %T and %T", k, other))
	}
	if k < o {
		return -1
	}
	if k > o {
		return 1
	}
	return 0
}

// ToKey converts a supported interface{} value into a strict Key.
func ToKey(val interface{}) Key {
	switch v := val.(type) {
	case Key:
		return v
	case int64:
		return IntKey(v)
	case int:
		return IntKey(int64(v))
	case float64:
		return FloatKey(v)
	case string:
		return StringKey(v)
	default:
		panic(fmt.Sprintf("btree: unsupported key type: %T", val))
	}
}

// compareKeys compares two keys. Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareKeys(a, b interface{}) int {
	return ToKey(a).Compare(ToKey(b))
}

// ---------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------

// Search returns the position for a key, or (0, false) if not found.
func (t *BPlusTree) Search(key interface{}) (pos int64, found bool) {
	n := t.root
	for !n.leaf {
		// Find the child to descend into.
		i := searchNode(n.keys, key)
		n = n.children[i]
	}
	// Linear/binary search in leaf.
	i, exact := leafSearch(n.keys, key)
	if !exact {
		return 0, false
	}
	return n.values[i], true
}

// searchNode returns the index of the child pointer to follow in an internal node.
// If key < keys[0], returns 0. If key >= keys[last], returns len(keys).
// Otherwise returns the first i such that key < keys[i].
func searchNode(keys []interface{}, key interface{}) int {
	lo, hi := 0, len(keys)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if compareKeys(key, keys[mid]) < 0 {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}

// leafSearch does a binary search in a leaf's keys slice.
// Returns (index, true) if found, or (insertion point, false) if not.
func leafSearch(keys []interface{}, key interface{}) (int, bool) {
	lo, hi := 0, len(keys)
	for lo < hi {
		mid := lo + (hi-lo)/2
		cmp := compareKeys(key, keys[mid])
		if cmp == 0 {
			return mid, true
		}
		if cmp < 0 {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo, false
}

// ---------------------------------------------------------------------
// Insert
// ---------------------------------------------------------------------

// Insert adds a key → pos mapping. Returns ErrDuplicateKey if key already exists.
func (t *BPlusTree) Insert(key interface{}, pos int64) error {
	// If root is full, split it first (proactive splitting at root).
	maxKeys := 2*t.degree - 1
	if len(t.root.keys) == maxKeys {
		oldRoot := t.root
		newRoot := &node{
			keys:     make([]interface{}, 0, maxKeys),
			children: make([]*node, 0, maxKeys+1),
			leaf:     false,
		}
		newRoot.children = append(newRoot.children, oldRoot)
		t.splitChild(newRoot, 0)
		t.root = newRoot
	}

	return t.insertNonFull(t.root, key, pos)
}

// insertNonFull inserts into a node that is guaranteed to have room.
func (t *BPlusTree) insertNonFull(n *node, key interface{}, pos int64) error {
	maxKeys := 2*t.degree - 1

	if n.leaf {
		// Find insertion point in leaf.
		i, found := leafSearch(n.keys, key)
		if found {
			return ErrDuplicateKey
		}
		// Insert key and value at position i.
		n.keys = append(n.keys, nil)
		copy(n.keys[i+1:], n.keys[i:])
		n.keys[i] = key

		n.values = append(n.values, 0)
		copy(n.values[i+1:], n.values[i:])
		n.values[i] = pos

		t.size++
		return nil
	}

	// Internal node: find child to descend into.
	i := searchNode(n.keys, key)

	// If that child is full, split it first.
	if len(n.children[i].keys) == maxKeys {
		t.splitChild(n, i)
		// After split, determine which of the two children to use.
		if compareKeys(key, n.keys[i]) >= 0 {
			i++
		}
	}

	return t.insertNonFull(n.children[i], key, pos)
}

// splitChild splits n.children[i] (which must be full) into two nodes
// and promotes the median key into n.
func (t *BPlusTree) splitChild(parent *node, i int) {
	full := parent.children[i]
	mid := t.degree - 1 // index of median key (0-based)

	// Create the new right sibling.
	right := &node{
		keys: make([]interface{}, 0, 2*t.degree-1),
		leaf: full.leaf,
	}

	if full.leaf {
		// Leaf split: copy keys[mid:] to right. Promote a copy of keys[mid].
		right.keys = append(right.keys, full.keys[mid:]...)
		right.values = make([]int64, 0, 2*t.degree-1)
		right.values = append(right.values, full.values[mid:]...)

		// Truncate left.
		full.keys = full.keys[:mid]
		full.values = full.values[:mid]

		// Maintain leaf linked list.
		right.next = full.next
		right.prev = full
		if full.next != nil {
			full.next.prev = right
		}
		full.next = right

		// Promote the first key of the right sibling (copy, not move).
		promoteKey := right.keys[0]

		// Insert promoted key and right child into parent.
		parent.keys = append(parent.keys, nil)
		copy(parent.keys[i+1:], parent.keys[i:])
		parent.keys[i] = promoteKey

		parent.children = append(parent.children, nil)
		copy(parent.children[i+2:], parent.children[i+1:])
		parent.children[i+1] = right
	} else {
		// Internal split: keys[mid] is promoted (removed from children).
		right.keys = append(right.keys, full.keys[mid+1:]...)
		right.children = make([]*node, 0, 2*t.degree)
		right.children = append(right.children, full.children[mid+1:]...)

		promoteKey := full.keys[mid]

		// Truncate left.
		full.keys = full.keys[:mid]
		full.children = full.children[:mid+1]

		// Insert promoted key and right child into parent.
		parent.keys = append(parent.keys, nil)
		copy(parent.keys[i+1:], parent.keys[i:])
		parent.keys[i] = promoteKey

		parent.children = append(parent.children, nil)
		copy(parent.children[i+2:], parent.children[i+1:])
		parent.children[i+1] = right
	}
}

// ---------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------

// Delete removes a key from the tree. Returns ErrKeyNotFound if the key
// does not exist.
func (t *BPlusTree) Delete(key interface{}) error {
	deleted := t.delete(t.root, key)
	if !deleted {
		return ErrKeyNotFound
	}
	t.size--

	// If root is an internal node with no keys, shrink the tree.
	if !t.root.leaf && len(t.root.keys) == 0 {
		t.root = t.root.children[0]
	}

	// Update first pointer if root is now a leaf.
	if t.root.leaf {
		t.first = t.root
	}

	return nil
}

// delete recursively removes key from the subtree rooted at n.
// Returns true if a key was deleted.
func (t *BPlusTree) delete(n *node, key interface{}) bool {
	if n.leaf {
		return t.deleteFromLeaf(n, key)
	}
	return t.deleteFromInternal(n, key)
}

// deleteFromLeaf removes a key from a leaf node.
func (t *BPlusTree) deleteFromLeaf(n *node, key interface{}) bool {
	i, found := leafSearch(n.keys, key)
	if !found {
		return false
	}
	// Remove key and value at index i.
	n.keys = append(n.keys[:i], n.keys[i+1:]...)
	n.values = append(n.values[:i], n.values[i+1:]...)
	return true
}

// deleteFromInternal handles deletion from an internal node's subtree,
// ensuring children maintain the minimum key count.
func (t *BPlusTree) deleteFromInternal(n *node, key interface{}) bool {
	i := searchNode(n.keys, key)

	child := n.children[i]
	minKeys := t.degree - 1

	// Ensure the child we descend into has more than minKeys.
	if len(child.keys) <= minKeys {
		t.fill(n, i)
		// After filling, the index may have changed. Re-find.
		// If merge happened, n.children may have shrunk.
		if i >= len(n.children) {
			i = len(n.children) - 1
		}
		child = n.children[i]
		// Re-check if key would be in this child or the next.
		if i < len(n.keys) && compareKeys(key, n.keys[i]) >= 0 {
			child = n.children[i+1]
			i = i + 1
		} else if i > 0 && compareKeys(key, n.keys[i-1]) < 0 {
			child = n.children[i-1]
			i = i - 1
		}
	}

	deleted := t.delete(child, key)
	if !deleted {
		return false
	}

	// After deletion from a leaf, the routing key in the parent may need updating.
	// If we deleted the first key of a non-first child, update the separator.
	t.updateSeparators(n)

	return true
}

// updateSeparators refreshes separator keys in an internal node so they
// match the actual first key of each right child. This is needed because
// in a B+Tree, internal keys are copies of leaf keys and leaf keys shift
// during deletes and merges.
func (t *BPlusTree) updateSeparators(n *node) {
	for i := 0; i < len(n.keys); i++ {
		firstKey := t.leftmostKey(n.children[i+1])
		if firstKey != nil {
			n.keys[i] = firstKey
		}
	}
}

// leftmostKey returns the first (smallest) key in the subtree rooted at n.
func (t *BPlusTree) leftmostKey(n *node) interface{} {
	for !n.leaf {
		n = n.children[0]
	}
	if len(n.keys) == 0 {
		return nil
	}
	return n.keys[0]
}

// fill ensures n.children[i] has more than minKeys by borrowing or merging.
func (t *BPlusTree) fill(n *node, i int) {
	minKeys := t.degree - 1

	// Try borrowing from left sibling.
	if i > 0 && len(n.children[i-1].keys) > minKeys {
		t.borrowFromLeft(n, i)
		return
	}

	// Try borrowing from right sibling.
	if i < len(n.children)-1 && len(n.children[i+1].keys) > minKeys {
		t.borrowFromRight(n, i)
		return
	}

	// Merge with a sibling.
	if i < len(n.children)-1 {
		t.mergeChildren(n, i) // merge i and i+1
	} else {
		t.mergeChildren(n, i-1) // merge i-1 and i
	}
}

// borrowFromLeft rotates a key from the left sibling through the parent.
func (t *BPlusTree) borrowFromLeft(parent *node, i int) {
	child := parent.children[i]
	leftSib := parent.children[i-1]

	if child.leaf {
		// Move the last key/value from left sibling to front of child.
		lastKey := leftSib.keys[len(leftSib.keys)-1]
		lastVal := leftSib.values[len(leftSib.values)-1]

		child.keys = append([]interface{}{lastKey}, child.keys...)
		child.values = append([]int64{lastVal}, child.values...)

		leftSib.keys = leftSib.keys[:len(leftSib.keys)-1]
		leftSib.values = leftSib.values[:len(leftSib.values)-1]

		// Update parent separator: it should be the new first key of child.
		parent.keys[i-1] = child.keys[0]
	} else {
		// Internal node: rotate through parent.
		child.keys = append([]interface{}{parent.keys[i-1]}, child.keys...)
		parent.keys[i-1] = leftSib.keys[len(leftSib.keys)-1]
		leftSib.keys = leftSib.keys[:len(leftSib.keys)-1]

		// Move the last child pointer.
		lastChild := leftSib.children[len(leftSib.children)-1]
		child.children = append([]*node{lastChild}, child.children...)
		leftSib.children = leftSib.children[:len(leftSib.children)-1]
	}
}

// borrowFromRight rotates a key from the right sibling through the parent.
func (t *BPlusTree) borrowFromRight(parent *node, i int) {
	child := parent.children[i]
	rightSib := parent.children[i+1]

	if child.leaf {
		// Move first key/value from right sibling to end of child.
		firstKey := rightSib.keys[0]
		firstVal := rightSib.values[0]

		child.keys = append(child.keys, firstKey)
		child.values = append(child.values, firstVal)

		rightSib.keys = rightSib.keys[1:]
		rightSib.values = rightSib.values[1:]

		// Update parent separator: it should be the new first key of right sibling.
		parent.keys[i] = rightSib.keys[0]
	} else {
		// Internal node: rotate through parent.
		child.keys = append(child.keys, parent.keys[i])
		parent.keys[i] = rightSib.keys[0]
		rightSib.keys = rightSib.keys[1:]

		// Move the first child pointer.
		firstChild := rightSib.children[0]
		child.children = append(child.children, firstChild)
		rightSib.children = rightSib.children[1:]
	}
}

// mergeChildren merges n.children[i+1] into n.children[i] and removes
// the separator key from n.
func (t *BPlusTree) mergeChildren(parent *node, i int) {
	left := parent.children[i]
	right := parent.children[i+1]

	if left.leaf {
		// Leaf merge: append right's keys/values to left.
		left.keys = append(left.keys, right.keys...)
		left.values = append(left.values, right.values...)

		// Fix linked list.
		left.next = right.next
		if right.next != nil {
			right.next.prev = left
		}
	} else {
		// Internal merge: pull down separator, then append right.
		left.keys = append(left.keys, parent.keys[i])
		left.keys = append(left.keys, right.keys...)
		left.children = append(left.children, right.children...)
	}

	// Remove separator and right child from parent.
	parent.keys = append(parent.keys[:i], parent.keys[i+1:]...)
	parent.children = append(parent.children[:i+1], parent.children[i+2:]...)

	// Update first pointer if needed.
	if t.first == right {
		t.first = left
	}
}

// ---------------------------------------------------------------------
// Range scans (via leaf linked list)
// ---------------------------------------------------------------------

// RangeScan returns all positions where lo <= key <= hi, in key order.
// Uses the leaf linked-list for O(log n + k) performance.
func (t *BPlusTree) RangeScan(lo, hi interface{}) []int64 {
	// Navigate to the leaf containing lo.
	n := t.root
	for !n.leaf {
		i := searchNode(n.keys, lo)
		n = n.children[i]
	}

	var result []int64
	// Walk the linked list starting from this leaf.
	for n != nil {
		for i, k := range n.keys {
			cmp := compareKeys(k, lo)
			if cmp < 0 {
				continue // skip keys before lo
			}
			if compareKeys(k, hi) > 0 {
				return result // past hi, done
			}
			result = append(result, n.values[i])
		}
		n = n.next
	}
	return result
}

// RangeFrom returns all positions where key >= lo, in key order.
// Uses the leaf linked-list for O(log n + k) performance.
func (t *BPlusTree) RangeFrom(lo interface{}) []int64 {
	// Navigate to the leaf containing lo.
	n := t.root
	for !n.leaf {
		i := searchNode(n.keys, lo)
		n = n.children[i]
	}

	var result []int64
	// Walk the linked list starting from this leaf.
	for n != nil {
		for i, k := range n.keys {
			cmp := compareKeys(k, lo)
			if cmp < 0 {
				continue // skip keys before lo
			}
			result = append(result, n.values[i])
		}
		n = n.next
	}
	return result
}

// RangeTo returns all positions where key <= hi, in key order.
// Uses the leaf linked-list for O(k) performance.
func (t *BPlusTree) RangeTo(hi interface{}) []int64 {
	var result []int64
	// Walk the linked list starting from the first leaf.
	n := t.first
	for n != nil {
		for i, k := range n.keys {
			if compareKeys(k, hi) > 0 {
				return result // past hi, done
			}
			result = append(result, n.values[i])
		}
		n = n.next
	}
	return result
}


// All returns all positions in key order by walking the leaf linked-list.
// This is O(n) — no tree traversal needed.
func (t *BPlusTree) All() []int64 {
	result := make([]int64, 0, t.size)
	n := t.first
	for n != nil {
		result = append(result, n.values...)
		n = n.next
	}
	return result
}

// ---------------------------------------------------------------------
// UpdatePos replaces the stored position for an existing key.
// Returns ErrKeyNotFound if the key does not exist.
// This is useful when row positions shift (e.g., after compaction).
// ---------------------------------------------------------------------

// UpdatePos updates the position associated with an existing key.
func (t *BPlusTree) UpdatePos(key interface{}, newPos int64) error {
	n := t.root
	for !n.leaf {
		i := searchNode(n.keys, key)
		n = n.children[i]
	}
	i, found := leafSearch(n.keys, key)
	if !found {
		return ErrKeyNotFound
	}
	n.values[i] = newPos
	return nil
}

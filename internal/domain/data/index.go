package data

// Index is an in-memory index on a single column
type Index struct {
	Column string
	Data   map[interface{}][]int // value → row positions
	Unique bool
}

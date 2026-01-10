package schema

// Database represents a single database on disk
// (a directory containing table subdirectories)
type Database struct {
	Name   string
	Path   string // filesystem path to database directory
	Tables map[string]*Table
}

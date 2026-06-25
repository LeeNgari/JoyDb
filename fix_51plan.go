package main

import (
	"fmt"
	"io/ioutil"
	"strings"
)

func main() {
	b, err := ioutil.ReadFile("5.1plan.md")
	if err != nil {
		fmt.Println(err)
		return
	}
	content := string(b)

	search := `## Phase 3: The Core Table Rewrite`
	replace := `## Phase 3: The Core Table Rewrite

- [x] Completed Phase 3: Rewrote schema.Table to use RowsByRID instead of []data.Row. Handled auto-increment logic, implemented LiveRows, Insert, Update, Delete with tombstoning, and updated select logic to rely on RIDs. Added strict method locking hierarchy.`

	content = strings.Replace(content, search, replace, 1)

	ioutil.WriteFile("5.1plan.md", []byte(content), 0644)
	fmt.Println("Done")
}

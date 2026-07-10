package workloads

import (
	"fmt"

	"github.com/leengari/joydb/internal/engine"
)

// CreateDropTable measures DDL overhead
type CreateDropTable struct{}

func NewCreateDropTable() *CreateDropTable { return &CreateDropTable{} }

func (w *CreateDropTable) Name() string { return "create_drop_table" }
func (w *CreateDropTable) Description() string { return "CREATE + DROP table per iteration. Measures DDL overhead." }
func (w *CreateDropTable) Tags() []string { return []string{"ddl", "schema"} }

func (w *CreateDropTable) Setup(eng *engine.Engine) error { return nil }

func (w *CreateDropTable) Run(eng *engine.Engine, iter int) error {
	tableName := fmt.Sprintf("test_ddl_%d", iter)
	
	createSQL := fmt.Sprintf(`CREATE TABLE %s (
		id INT PRIMARY KEY AUTO_INCREMENT,
		name TEXT
	)`, tableName)
	
	if _, err := eng.Execute(createSQL); err != nil {
		return err
	}
	
	dropSQL := fmt.Sprintf("DROP TABLE %s", tableName)
	_, err := eng.Execute(dropSQL)
	return err
}

func (w *CreateDropTable) Teardown(eng *engine.Engine) error { return nil }

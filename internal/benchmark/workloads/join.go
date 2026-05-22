package workloads

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/engine"
)

// JoinNestedLoop100x100 measures JOIN performance on small tables
type JoinNestedLoop100x100 struct{}

func NewJoinNestedLoop100x100() *JoinNestedLoop100x100 { return &JoinNestedLoop100x100{} }
func (w *JoinNestedLoop100x100) Name() string { return "join_nested_loop_100_x_100" }
func (w *JoinNestedLoop100x100) Description() string { return "JOIN two 100-row tables. Establishes baseline for small joins." }
func (w *JoinNestedLoop100x100) Tags() []string { return []string{"read", "join", "nested-loop"} }

func (w *JoinNestedLoop100x100) Setup(eng *engine.Engine) error {
	if _, err := eng.Execute(`CREATE TABLE users100 (id INT PRIMARY KEY AUTO_INCREMENT, name TEXT)`); err != nil {
		return err
	}
	if _, err := eng.Execute(`CREATE TABLE orders100 (id INT PRIMARY KEY AUTO_INCREMENT, user_id INT, product TEXT)`); err != nil {
		return err
	}

	for i := 1; i <= 100; i++ {
		eng.Execute(fmt.Sprintf("INSERT INTO users100 (name) VALUES ('user%d')", i))
		eng.Execute(fmt.Sprintf("INSERT INTO orders100 (user_id, product) VALUES (%d, 'prod%d')", i, i))
	}
	return nil
}

func (w *JoinNestedLoop100x100) Run(eng *engine.Engine, iter int) error {
	_, err := eng.Execute("SELECT * FROM users100 JOIN orders100 ON users100.id = orders100.user_id")
	return err
}

func (w *JoinNestedLoop100x100) Teardown(eng *engine.Engine) error {
	eng.Execute("DROP TABLE orders100")
	eng.Execute("DROP TABLE users100")
	return nil
}

// JoinNestedLoop1Kx1K measures JOIN performance on larger tables
type JoinNestedLoop1Kx1K struct{}

func NewJoinNestedLoop1Kx1K() *JoinNestedLoop1Kx1K { return &JoinNestedLoop1Kx1K{} }
func (w *JoinNestedLoop1Kx1K) Name() string { return "join_nested_loop_1k_x_1k" }
func (w *JoinNestedLoop1Kx1K) Description() string { return "JOIN two 1K-row tables. Shows O(n^2) scaling." }
func (w *JoinNestedLoop1Kx1K) Tags() []string { return []string{"read", "join", "nested-loop"} }

func (w *JoinNestedLoop1Kx1K) Setup(eng *engine.Engine) error {
	if _, err := eng.Execute(`CREATE TABLE users1k (id INT PRIMARY KEY AUTO_INCREMENT, name TEXT)`); err != nil {
		return err
	}
	if _, err := eng.Execute(`CREATE TABLE orders1k (id INT PRIMARY KEY AUTO_INCREMENT, user_id INT, product TEXT)`); err != nil {
		return err
	}

	for i := 1; i <= 1000; i++ {
		eng.Execute(fmt.Sprintf("INSERT INTO users1k (name) VALUES ('user%d')", i))
		eng.Execute(fmt.Sprintf("INSERT INTO orders1k (user_id, product) VALUES (%d, 'prod%d')", i, i))
	}
	return nil
}

func (w *JoinNestedLoop1Kx1K) Run(eng *engine.Engine, iter int) error {
	_, err := eng.Execute("SELECT * FROM users1k JOIN orders1k ON users1k.id = orders1k.user_id")
	return err
}

func (w *JoinNestedLoop1Kx1K) Teardown(eng *engine.Engine) error {
	eng.Execute("DROP TABLE orders1k")
	eng.Execute("DROP TABLE users1k")
	return nil
}

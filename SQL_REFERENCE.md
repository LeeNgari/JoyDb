# JoyDB SQL Syntax Reference

## Overview

This document describes the SQL syntax supported by JoyDB. The system supports a subset of standard SQL with full CRUD operations,  filtering, and JOIN capabilities.

---

## Quick Start

### Common Query Patterns

```sql
-- Database setup
CREATE DATABASE myapp;
USE myapp;

-- View all data
SELECT * FROM users;

-- Find specific records
SELECT * FROM users WHERE id = 5;
SELECT * FROM users WHERE username = 'alice';

-- Insert new data
INSERT INTO users (id, username, email) VALUES (100, 'bob', 'bob@example.com');

-- Update existing data
UPDATE users SET email = 'newemail@example.com' WHERE id = 100;

-- Delete records
DELETE FROM users WHERE id = 100;

-- Join tables
SELECT users.username, orders.product 
FROM users 
INNER JOIN orders ON users.id = orders.user_id;
```

### Common Commands

| Operation | Template | Example |
|-----------|----------|---------|
| **Select all** | `SELECT * FROM table;` | `SELECT * FROM users;` |
| **Filter** | `SELECT * FROM table WHERE col = value;` | `SELECT * FROM users WHERE age > 18;` |
| **Insert** | `INSERT INTO table (cols) VALUES (vals);` | `INSERT INTO users (id, name) VALUES (1, 'Alice');` |
| **Update** | `UPDATE table SET col = val WHERE condition;` | `UPDATE users SET active = true WHERE id = 5;` |
| **Delete** | `DELETE FROM table WHERE condition;` | `DELETE FROM users WHERE id = 5;` |
| **Join** | `SELECT * FROM t1 JOIN t2 ON t1.id = t2.fk;` | `SELECT * FROM users JOIN orders ON users.id = orders.user_id;` |

---

## Supported SQL Statements

### 1. Database Management

#### CREATE DATABASE
Creates a new database.
```sql
CREATE DATABASE my_database;
```

#### USE
Switches the active database context.
```sql
USE my_database;
```

#### DROP DATABASE
Deletes a database and all its tables.
```sql
DROP DATABASE my_database;
```

---

### 2. SELECT Statement

#### Basic Syntax
```sql
SELECT column1, column2, ... FROM table_name;
SELECT * FROM table_name;
```

#### With WHERE Clause
```sql
SELECT column1, column2 FROM table_name WHERE condition;
```

#### With Qualified Column Names
```sql
SELECT table1.column1, table2.column2 FROM table1 JOIN table2 ON ...;
```

#### Examples
```sql
-- Select all columns
SELECT * FROM users;

-- Select specific columns
SELECT username, email FROM users;

-- Select with qualified names
SELECT users.username, users.email FROM users;

-- Select with WHERE
SELECT * FROM users WHERE id = 5;
SELECT username, email FROM users WHERE is_active = true;
```

---

### 3. INSERT Statement

#### Syntax
```sql
INSERT INTO table_name (column1, column2, ...) VALUES (value1, value2, ...);
```

#### Examples
```sql
-- Insert a new user
INSERT INTO users (id, username, email) VALUES (100, 'alice', 'alice@example.com');

-- Insert with boolean
INSERT INTO users (id, username, email, is_active) VALUES (101, 'bob', 'bob@example.com', true);

-- Insert with NULL (use keyword)
INSERT INTO users (id, username, email) VALUES (102, 'charlie', NULL);
```

---

### 4. UPDATE Statement

#### Syntax
```sql
UPDATE table_name SET column1 = value1, column2 = value2, ... WHERE condition;
```

#### Examples
```sql
-- Update single column
UPDATE users SET email = 'newemail@example.com' WHERE id = 5;

-- Update multiple columns
UPDATE users SET email = 'updated@example.com', is_active = false WHERE id = 10;

-- Update with comparison
UPDATE users SET is_active = false WHERE id > 100;

-- Update all rows (no WHERE clause)
UPDATE users SET is_active = true;
```

---

### 5. DELETE Statement

#### Syntax
```sql
DELETE FROM table_name WHERE condition;
```

#### Examples
```sql
-- Delete specific row
DELETE FROM users WHERE id = 999;

-- Delete with string comparison
DELETE FROM users WHERE username = 'testuser';

-- Delete with condition
DELETE FROM logs WHERE timestamp < 1000000;

-- Delete all rows (use with caution!)
DELETE FROM temp_table;
```

---

## WHERE Clause Conditions

### Comparison Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `=` | Equal to | `WHERE age = 25` |
| `!=` | Not equal to | `WHERE status != 'inactive'` |
| `<>` | Not equal to (alternative) | `WHERE status <> 'deleted'` |
| `<` | Less than | `WHERE age < 18` |
| `>` | Greater than | `WHERE price > 100` |
| `<=` | Less than or equal | `WHERE age <= 65` |
| `>=` | Greater than or equal | `WHERE price >= 50` |

### Logical Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `AND` | Both conditions must be true | `WHERE age > 18 AND active = true` |
| `OR` | Either condition must be true | `WHERE status = 'pending' OR status = 'processing'` |

### Operator Precedence
1. **Comparison operators** (=, <, >, <=, >=, !=, <>) - Highest precedence
2. **AND** - Higher precedence than OR
3. **OR** - Lowest precedence

Use parentheses `()` to override precedence:
```sql
WHERE (age > 18 OR premium = true) AND active = true
```

### Examples
```sql
-- Simple comparison
SELECT * FROM users WHERE age > 18;

-- Multiple conditions with AND
SELECT * FROM users WHERE age >= 18 AND is_active = true;

-- Multiple conditions with OR
SELECT * FROM orders WHERE status = 'pending' OR status = 'processing';

-- Complex conditions with parentheses
SELECT * FROM users WHERE (age < 18 OR age > 65) AND verified = true;

-- Qualified column names
SELECT * FROM orders WHERE orders.amount > 100;
```

---

## JOIN Operations

### Supported JOIN Types

| JOIN Type | Description |
|-----------|-------------|
| `INNER JOIN` | Returns only matching rows from both tables |
| `LEFT JOIN` / `LEFT OUTER JOIN` | Returns all rows from left table, matching rows from right (NULL if no match) |
| `RIGHT JOIN` / `RIGHT OUTER JOIN` | Returns all rows from right table, matching rows from left (NULL if no match) |
| `FULL JOIN` / `FULL OUTER JOIN` | Returns all rows from both tables (NULL where no match) |

### Syntax
```sql
SELECT columns
FROM table1
[INNER|LEFT|RIGHT|FULL] [OUTER] JOIN table2 
ON table1.column = table2.column
[WHERE condition];
```

### Examples

#### INNER JOIN
```sql
-- Basic INNER JOIN
SELECT * FROM users 
INNER JOIN orders ON users.id = orders.user_id;

-- INNER JOIN with specific columns
SELECT users.username, orders.product, orders.amount 
FROM users 
INNER JOIN orders ON users.id = orders.user_id;

-- INNER JOIN with WHERE clause
SELECT users.username, orders.product 
FROM users 
INNER JOIN orders ON users.id = orders.user_id 
WHERE orders.amount > 100;
```

#### LEFT JOIN
```sql
-- LEFT JOIN (includes users without orders)
SELECT users.username, orders.product 
FROM users 
LEFT JOIN orders ON users.id = orders.user_id;

-- LEFT JOIN with WHERE
SELECT users.username, orders.product 
FROM users 
LEFT JOIN orders ON users.id = orders.user_id 
WHERE users.is_active = true;
```

#### RIGHT JOIN
```sql
-- RIGHT JOIN (includes orders without users)
SELECT users.username, orders.product 
FROM users 
RIGHT JOIN orders ON users.id = orders.user_id;
```

#### FULL OUTER JOIN
```sql
-- FULL JOIN (includes all users and all orders)
SELECT users.username, orders.product 
FROM users 
FULL OUTER JOIN orders ON users.id = orders.user_id;
```

---

## Data Types

### Supported Literal Types

| Type | Example | Description |
|------|---------|-------------|
| **Integer** | `42`, `0`, `-10` | Whole numbers |
| **Float** | `3.14`, `99.99`, `-0.5` | Decimal numbers |
| **String** | `'hello'`, `'user@example.com'` | Text enclosed in single quotes |
| **Boolean** | `true`, `false` | Boolean values (case-insensitive) |

### Type Comparison Rules
- **Numeric types** (int, float) are compared numerically
- **Strings** are compared lexicographically (alphabetically)
- **Booleans** support equality/inequality only (=, !=, <>)

---

## Complete Query Examples

### Simple Queries
```sql
-- View all users
SELECT * FROM users;

-- Find specific user
SELECT * FROM users WHERE username = 'alice';

-- Find active users
SELECT username, email FROM users WHERE is_active = true;

-- Find users by age range
SELECT * FROM users WHERE age >= 18 AND age <= 65;
```

### CRUD Operations
```sql
-- Create a new user
INSERT INTO users (id, username, email, is_active) 
VALUES (200, 'newuser', 'new@example.com', true);

-- Read user data
SELECT * FROM users WHERE id = 200;

-- Update user
UPDATE users SET email = 'updated@example.com' WHERE id = 200;

-- Delete user
DELETE FROM users WHERE id = 200;
```

### Complex Filtering
```sql
-- Multiple conditions
SELECT * FROM users 
WHERE (age > 18 AND verified = true) OR premium = true;

-- Range queries
SELECT * FROM products 
WHERE price >= 10 AND price <= 100 AND stock > 0;

-- Pattern matching with comparisons
SELECT * FROM orders 
WHERE status != 'cancelled' AND amount > 50;
```

### JOIN Queries
```sql
-- Find all user orders
SELECT users.username, orders.product, orders.amount 
FROM users 
INNER JOIN orders ON users.id = orders.user_id;

-- Find users with expensive orders
SELECT users.username, orders.product, orders.amount 
FROM users 
INNER JOIN orders ON users.id = orders.user_id 
WHERE orders.amount > 500;

-- Find all users and their orders (including users without orders)
SELECT users.username, orders.product 
FROM users 
LEFT JOIN orders ON users.id = orders.user_id;

-- Complex JOIN with multiple conditions
SELECT users.username, orders.product, orders.amount 
FROM users 
INNER JOIN orders ON users.id = orders.user_id 
WHERE users.is_active = true AND orders.amount > 100;
```

---

## Limitations & Notes

### Current Limitations
1. **Single JOIN only**: Multiple JOINs in one query not yet supported
2. **No aggregate functions**: SUM, COUNT, AVG, MIN, MAX not supported
3. **No GROUP BY / HAVING**: Grouping operations not supported
4. **No ORDER BY**: Result ordering not supported
5. **No LIMIT / OFFSET**: Pagination not supported
6. **No subqueries**: Nested SELECT statements not supported
7. **No DISTINCT**: Duplicate removal not supported
8. **Literal values only in SET**: UPDATE SET clause only supports literal values, not expressions



### Statement Termination
- Semicolons (`;`) are **optional** at the end of statements
- Both `SELECT * FROM users;` and `SELECT * FROM users` are valid

---

## REPL Commands

### Special Commands
- `exit` or `\q` - Exit the REPL
- Queries are executed immediately after pressing Enter

### Example REPL Session
```
> SELECT * FROM users;
Returned 4 rows
id   username   email                is_active
---  ---        ---                  ---
2    bob        bob@example.com      true
5    eve        eve@example.com      true

> UPDATE users SET is_active = false WHERE id = 5;
UPDATE 1

> SELECT * FROM users WHERE is_active = true;
Returned 1 rows
id   username   email                is_active
---  ---        ---                  ---
2    bob        bob@example.com      true

> exit
```




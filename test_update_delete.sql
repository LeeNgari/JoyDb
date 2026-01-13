-- Test UPDATE and DELETE statements in REPL

-- First, check current users
SELECT * FROM users;

-- Test UPDATE single column
UPDATE users SET email = 'repl_updated@test.com' WHERE id = 18;

-- Verify UPDATE
SELECT email FROM users WHERE id = 18;

-- Test UPDATE multiple columns
UPDATE users SET email = 'bob_updated@test.com', username = 'bob_new' WHERE id = 2;

-- Verify multi-column UPDATE
SELECT username, email FROM users WHERE id = 2;

-- Test UPDATE with boolean
UPDATE users SET is_active = false WHERE id = 6;

-- Verify boolean UPDATE
SELECT is_active FROM users WHERE id = 6;

-- Test INSERT for DELETE testing
INSERT INTO users (id, username, email) VALUES (1000, 'delete_me', 'delete@test.com');

-- Verify INSERT
SELECT * FROM users WHERE id = 1000;

-- Test DELETE
DELETE FROM users WHERE id = 1000;

-- Verify DELETE
SELECT * FROM users WHERE id = 1000;

-- Test DELETE with string WHERE
INSERT INTO users (id, username, email) VALUES (1001, 'another_delete', 'another@test.com');
DELETE FROM users WHERE username = 'another_delete';

-- Final check
SELECT * FROM users;

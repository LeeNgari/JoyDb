- Add Central Lock Manager: track fine-grained row-level locks by their logical RID instead of wrapping table storage with coarse-grained locks.


Milestone 4: Analytical SQL
Real applications need analytics and pagination.

Pagination: Implement LIMIT and OFFSET in the Parser and Planner.
Aggregations: Implement COUNT(), SUM(), and GROUP BY.
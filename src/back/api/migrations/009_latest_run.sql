-- Every task the board shows reads its latest test run, and the move rules read it too.
create index on test_runs (task_id, id);

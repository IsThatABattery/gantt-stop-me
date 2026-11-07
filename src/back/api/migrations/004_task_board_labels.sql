-- The issue's board:* labels as the board last saw them, so a sync moves a task only for a label added since.
alter table tasks add column board_labels text[] not null default '{}';

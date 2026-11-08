alter table test_runs add column commented_at timestamptz; -- the run's issue comment was posted
alter table workflow_executions add column timeout interval not null default '30 minutes'; -- the workflow's, from the claim on
alter table test_results add column command text; -- a script's command; null for an LLM check

-- Runs that finished before comments existed get none.
update test_runs set commented_at = finished_at where finished_at is not null;

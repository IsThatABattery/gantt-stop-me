-- provider_id and model stay the interviewer's; the task agent gets its own, starting as the interviewer's.
alter table planning_settings
  add column task_provider_id         bigint references providers, -- a provider in use cannot be deleted; null with task_model for none
  add column task_model               text,
  add column interviewer_instructions text not null default '',    -- the creator's own, added to the interviewer's prompt
  add column task_instructions        text not null default '';
update planning_settings set task_provider_id = provider_id, task_model = model;
-- An agent has a provider and a model, or neither: the worker scans both into one model or fails the turn.
alter table planning_settings
  add constraint interviewer_model check ((provider_id is null) = (model is null)),
  add constraint task_agent_model check ((task_provider_id is null) = (task_model is null));

alter table project_automation
  add column decision_instructions text not null default '', -- the owner's own, added to the decision model's questions
  add column chat_instructions     text not null default ''; -- the owner's own, added to each LLM check's prompt

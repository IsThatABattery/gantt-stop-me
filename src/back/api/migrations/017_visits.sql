-- When the member last opened the project's board, which every project page loads; it orders their projects.
alter table project_members add column visited_at timestamptz not null default now();
update project_members set visited_at = checked_at;

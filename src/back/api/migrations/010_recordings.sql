alter table test_results add column video bytea; -- an LLM check's WebM recording of the browser, at most 64 MiB
alter table test_results alter column video set storage external; -- uncompressed, so substring() reads only its range

-- Persist §3.3 file-filtering rules per job as a JSON blob. Schema-on-read so
-- the rule shape can evolve without further migrations; internal/filter owns
-- the validation. Default '{}' reproduces the pre-feature behaviour: no rules
-- and ExcludeHidden=false, so legacy jobs continue to sync every file.
ALTER TABLE jobs ADD COLUMN filters_json TEXT NOT NULL DEFAULT '{}';

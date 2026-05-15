-- Add a flag to force a password change on first login.
-- Set true only for the seeded default admin (handled in code); existing rows
-- default to false so currently-running deployments are not affected.
ALTER TABLE users ADD COLUMN password_change_required BOOLEAN NOT NULL DEFAULT 0;

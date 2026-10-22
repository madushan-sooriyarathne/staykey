-- The role the API and cmd/migrate connect as in production. Superusers and BYPASSRLS roles skip
-- row-level security, so the API refuses to start on one. Run once as the provider's admin user,
-- with a fresh password from `openssl rand -hex 24` passed as the psql variable `pw`:
--
--   psql "$ADMIN_DATABASE_URL" -v pw=<generated> -f scripts/db-app-role.sql
--
-- then build DATABASE_URL from the role and that password. The role owns the database, so
-- migrations create tables it owns; the policies are forced, which keeps the owner behind them.

CREATE ROLE staykey LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEROLE PASSWORD :'pw';
CREATE DATABASE staykey OWNER staykey;

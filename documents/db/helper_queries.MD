## Queries to delete all the db tables:

```
DO $$ DECLARE
    r RECORD;
BEGIN
    -- if the schema you operate on is not "public", you will want to
    -- replace public with the name of the schema you are operating on.
    FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
        EXECUTE 'DROP TABLE IF EXISTS public.' || quote_ident(r.tablename) || ' CASCADE';
    END LOOP;
END $$;

```
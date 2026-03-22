# Migratex

A schema diff and migration engine that sits on top of your ORM. it treats database changes like a graph not a numbered list enabling deterministic migrations, drift detection, and safe CI/CD workflows.

works with **drizzle**, **prisma**, and **typeorm**. supports **postgres** and **mysql**.

## Why

every ORM handles migrations the same way: numbered files in a folder. `001_create_users.sql`, `002_add_posts.sql`, and so on. works fine solo. put a team on it and two devs on different branches will both create `004_*.sql`, someone has to manually rename theirs, and you pray the SQL doesn't conflict. at scale this is a constant source of broken deploys, merge pain, and wasted time.

migratex fixes this by replacing linear migrations with a DAG (like git uses for commits) and auto-generating the SQL from your ORM schema.

## features

### auto-generated migrations
stop writing SQL by hand. migratex reads your ORM schema, introspects the database, diffs them, and generates `up.sql` and `down.sql` automatically. change your schema file, run one command, done.

> **the problem this solves:** hand-written migrations drift from the ORM schema over time. someone forgets to add an index in the migration that they added in the schema. or the migration SQL has a typo that doesn't match what the ORM expects. migratex eliminates this entire class of bugs.

### DAG-based migration graph
migrations are nodes in a graph with content-addressed IDs (SHA-256 hashes), not numbered files. two branches can create migrations independently and merge cleanly — no renaming, no ordering conflicts.

> **the problem this solves:** on any team with parallel feature branches, linear migrations constantly collide. devs waste time renaming files, resolving fake conflicts, and coordinating who gets the next number. the DAG makes this a non-issue.

### conflict detection
migratex understands the schema semantically. if two branches add a `status` column to the same table with different types, it tells you. if they touch different tables, it merges cleanly. real conflicts get caught, false positives don't.

> **the problem this solves:** with linear migrations, you only find out about real schema conflicts when the migration runs (or worse, in production). migratex catches them at merge time, before anything touches the database.

### drift detection
compares your ORM schema against the actual database. catches out-of-band changes — someone ran `ALTER TABLE` directly on prod at 2am, or a migration was applied manually and never committed.

> **the problem this solves:** shadow changes to the database that nobody knows about until something breaks. "it works on my machine" but prod has an extra column nobody added through migrations.

### CI-friendly
`migratex check` validates the migration graph and detects drift. drop it in your pipeline — it exits non-zero if anything is wrong.

> **the problem this solves:** broken migrations making it to production because there's no automated gate. migratex check catches graph issues, missing parents, cycles, and schema drift before deploy.

### advisory locking
`migratex apply` uses database advisory locks. safe to run from multiple instances — no double-applying, no race conditions.

### up and down migrations
every migration automatically gets a reverse. rollbacks are generated, not hand-written.

## install

```bash
npm install migratex
```

## quickstart

### init

```bash
$ migratex init

Initializing migratex...

Detected ORM: drizzle              # or prisma, typeorm — auto-detected from package.json
Detected database: pg (from drizzle config)
Schema path [./db/schema]:

Created migratex.config.yaml
Created migrations/

migratex will read the database connection from your drizzle config at runtime.
```

no connection setup needed — migratex auto-detects your ORM from `package.json`, reads the dialect from your ORM config (`drizzle.config.ts`, `schema.prisma`, `ormconfig.json`), and uses the same env vars your ORM already uses for the database connection.

### generate

change your ORM schema, then:

```bash
$ migratex generate -m "add users table"

Reading ORM schema...
Introspecting database...
Computing diff...

Found 2 change(s):
  create_table users
  create_index users_email_idx

Generated migration: a1b2c3d4e5f6
  migrations/a1b2c3d4e5f6/up.sql
  migrations/a1b2c3d4e5f6/down.sql
  Description: add users table
```

### apply

```bash
$ migratex apply

Loading migration graph...
Acquiring lock...
Applying a1b2c3d4e5f6 — add users table... done
Releasing lock...

1 migration applied.
```

### check

```bash
$ migratex check

Validating migration graph... ok
Checking for drift... ok
No issues found.
```

or when something is wrong:

```bash
$ migratex check

Validating migration graph... ok
Checking for drift... DRIFT DETECTED

  Table "users":
    - Column "phone" exists in database but not in ORM schema
    - Column "email" has type "text" in database but "varchar(255)" in ORM schema

1 table has drifted from the expected schema.
```

### status

```bash
$ migratex status

Migration graph:
  a1b2c3d4 — create users table        [applied]
  d4e5f6a7 — add posts table           [applied]
  g7h8i9j0 — add sessions table        [pending]

Heads: g7h8i9j0
Applied: 2 | Pending: 1
```

## how it works

```
your ORM schema (TypeScript)
        |
        v
  sidecar extracts it as JSON
        |
        v
  go core diffs it against the actual DB
        |
        v
  generates migration SQL (up + down)
        |
        v
  stores it as a DAG node in migrations/
```

when you run `migratex generate`, it creates a folder structure like this:

```
migrations/
├── graph.json                          # tracks the DAG — heads, node metadata
│
├── a1b2c3d4e5f6/                       # first migration
│   ├── migration.json                  # parents, operations, checksums
│   ├── up.sql                          # apply this migration
│   └── down.sql                        # revert this migration
│
├── d4e5f6a7b8c9/                       # second migration (child of first)
│   ├── migration.json
│   ├── up.sql
│   └── down.sql
│
└── f7g8h9i0j1k2/                       # merge migration (two parents)
    ├── migration.json
    ├── up.sql
    └── down.sql
```

each folder is one migration node. the folder name is the migration ID — a truncated SHA-256 hash, not a sequential number. `graph.json` at the root ties them together into the DAG.

each migration node has:
- **content-addressed ID** — SHA-256 of (parent IDs + operations), truncated to 12 hex chars
- **parent pointers** — like git commits, can have multiple parents for merges
- **checksum** — SHA-256 of the up SQL, detects tampering
- **operations** — structured diff, so conflicts can be detected semantically

## config

```yaml
# migratex.config.yaml
orm: drizzle          # drizzle | prisma | typeorm
dialect: pg           # pg | mysql
schemaPath: ./db/schema
migrationsDir: ./migrations
```

no connection field needed — migratex reads it from your ORM's config at runtime. it checks `drizzle.config.ts`, `schema.prisma`, or `ormconfig.json` and uses the same env vars your ORM already uses (`DATABASE_URL`, `PGHOST`, etc.).

## license

MIT

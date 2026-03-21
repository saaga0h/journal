# TODO

## Schema

- Rename `repository` column in `journal_entries` to `source` (or similar) — the field now holds WebDAV filenames and other non-repository sources, not just git repository names. Requires a migration and updating all references in Go code and Makefile queries.

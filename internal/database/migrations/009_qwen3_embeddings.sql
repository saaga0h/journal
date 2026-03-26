-- Migration 009: Resize embedding columns from 768 to 4096 dimensions
-- for qwen3-embedding:8b model. NULLs all existing embeddings and clears
-- associations to force full re-embed + reassociate.

-- NULL existing embeddings first — ALTER TYPE cannot cast vector(768) to vector(4096)
UPDATE standing_documents SET embedding = NULL;
UPDATE journal_entries SET embedding = NULL;

ALTER TABLE standing_documents ALTER COLUMN embedding TYPE vector(4096);
ALTER TABLE journal_entries ALTER COLUMN embedding TYPE vector(4096);

-- Clear stale associations (dimension mismatch would produce wrong similarities)
DELETE FROM entry_standing_associations;

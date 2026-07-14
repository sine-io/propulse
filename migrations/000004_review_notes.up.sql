-- 复盘记录与看房笔记数据模型（WATCH-006.1 / #58）。
-- 单表按 kind 区分「复盘」与「看房笔记」，关联稳定用户身份与可选小区/周次。
CREATE TABLE IF NOT EXISTS review_notes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id TEXT NOT NULL CHECK (char_length(user_id) BETWEEN 1 AND 128),
  neighborhood_id UUID REFERENCES neighborhoods(id) ON DELETE SET NULL,
  kind TEXT NOT NULL CHECK (kind IN ('review', 'viewing_note')),
  week_start_date DATE,
  content TEXT NOT NULL CHECK (char_length(content) BETWEEN 1 AND 8000),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_review_notes_user_created_at
  ON review_notes(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_review_notes_user_neighborhood
  ON review_notes(user_id, neighborhood_id, created_at DESC);

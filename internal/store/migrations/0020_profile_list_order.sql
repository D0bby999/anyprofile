-- SPDX-License-Identifier: AGPL-3.0-or-later
-- Match the stable default paging order, including an ID tie-breaker when
-- profiles are created in the same second. Search still uses substring scans.
CREATE INDEX profiles_list_order ON profiles(deleted_at, pinned DESC, sort_order, created_at, id);

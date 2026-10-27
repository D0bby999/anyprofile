// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import "strings"

// The list used to load every account and extension in the database and
// join them in memory, so listing ten profiles cost as much as listing
// ten thousand. These loaders fetch only the rows of the profiles being
// listed. IDs are chunked to stay well under SQLite's variable limit.

const idChunk = 400

// inClause builds "col IN (?,?,…)" for one chunk and its args.
func inClause(col string, ids []string) (string, []any) {
	args := make([]any, len(ids))
	marks := make([]string, len(ids))
	for i, id := range ids {
		args[i], marks[i] = id, "?"
	}
	return col + " IN (" + strings.Join(marks, ",") + ")", args
}

// accountsByIDs loads the accounts of exactly the given profiles.
func (s *Store) accountsByIDs(ids []string) (map[string][]Account, error) {
	out := map[string][]Account{}
	for start := 0; start < len(ids); start += idChunk {
		end := min(start+idChunk, len(ids))
		clause, args := inClause("profile_id", ids[start:end])
		rows, err := s.db.Query(`SELECT profile_id, `+accountCols+` FROM accounts WHERE `+clause+` ORDER BY rowid`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			pid, a, err := scanAccount(rows)
			if err != nil {
				rows.Close()
				return nil, err
			}
			out[pid] = append(out[pid], a)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return out, nil
}

// extensionsByIDs loads the extension ids of exactly the given profiles.
func (s *Store) extensionsByIDs(ids []string) (map[string][]string, error) {
	out := map[string][]string{}
	for start := 0; start < len(ids); start += idChunk {
		end := min(start+idChunk, len(ids))
		clause, args := inClause("profile_id", ids[start:end])
		rows, err := s.db.Query(`SELECT profile_id, extension_id FROM profile_extensions WHERE `+clause+` ORDER BY rowid`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var pid, eid string
			if err := rows.Scan(&pid, &eid); err != nil {
				rows.Close()
				return nil, err
			}
			out[pid] = append(out[pid], eid)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return out, nil
}

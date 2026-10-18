// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// CopyTo writes a consistent copy of the database to path (a new file).
func (s *Store) CopyTo(path string) error {
	_, err := s.db.Exec(`VACUUM INTO ?`, path)
	return err
}

// ErrNewerDatabase: the other database was made by a newer AnyProfile.
var ErrNewerDatabase = errors.New("the database is from a newer version of AnyProfile")

// MergedAccount is an account Merge added; its secrets are under SecretKey.
type MergedAccount struct {
	SecretKey string
	Password  bool
	TOTP      bool
}

// MergeResult is what Merge added, and how much was already here.
type MergeResult struct {
	Profiles   []string // ids
	Proxies    []string
	Extensions []string
	Sites      []string
	Accounts   []MergedAccount

	ProfilesHere int // in both databases: left as they are here
	ProxiesHere  int
}

// Merge adds what the database at path has and this one does not: proxies,
// profiles with their accounts and extensions, custom platforms, extensions.
// Nothing here is changed or removed. Added profiles get the next serial
// numbers. Sessions, snapshots and settings stay behind: they belong to the
// computer they were made on.
//
// The other database must be at this one's schema (Open migrates it).
// beforeCommit runs inside the transaction with what is about to be added;
// an error from it adds nothing.
func (s *Store) Merge(ctx context.Context, path string, beforeCommit func(MergeResult) error) (MergeResult, error) {
	var res MergeResult
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return res, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS bak`, path); err != nil {
		return res, err
	}
	defer conn.ExecContext(context.Background(), `DETACH DATABASE bak`)
	var mine, theirs int
	if err := conn.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM main.schema_version`).Scan(&mine); err != nil {
		return res, err
	}
	if err := conn.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM bak.schema_version`).Scan(&theirs); err != nil {
		return res, err
	}
	if theirs > mine {
		return res, ErrNewerDatabase
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	defer tx.Rollback()

	newIDs := func(table string) ([]string, int, error) {
		ids, err := strs(ctx, tx, `SELECT id FROM bak.`+table+` WHERE id NOT IN (SELECT id FROM main.`+table+`) ORDER BY rowid`)
		if err != nil {
			return nil, 0, err
		}
		var all int
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM bak.`+table).Scan(&all)
		return ids, all - len(ids), err
	}
	copyRows := func(table, where string, args ...any) error {
		cols, err := sharedCols(ctx, tx, table)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO main.`+table+` (`+cols+`) SELECT `+cols+` FROM bak.`+table+` WHERE `+where, args...)
		return err
	}

	if res.Proxies, res.ProxiesHere, err = newIDs("proxies"); err != nil {
		return res, err
	}
	if res.Sites, _, err = newIDs("custom_sites"); err != nil {
		return res, err
	}
	if res.Extensions, _, err = newIDs("extensions"); err != nil {
		return res, err
	}
	for _, t := range []string{"proxies", "custom_sites", "extensions"} {
		if err := copyRows(t, `id NOT IN (SELECT id FROM main.`+t+`)`); err != nil {
			return res, fmt.Errorf("merge %s: %w", t, err)
		}
	}

	// Profiles one by one: each takes the next serial, and brings its accounts
	// and extensions.
	res.Profiles, err = strs(ctx, tx, `SELECT id FROM bak.profiles WHERE id NOT IN (SELECT id FROM main.profiles) ORDER BY serial`)
	if err != nil {
		return res, err
	}
	var all int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM bak.profiles`).Scan(&all); err != nil {
		return res, err
	}
	res.ProfilesHere = all - len(res.Profiles)
	cols, err := sharedCols(ctx, tx, "profiles", "serial")
	if err != nil {
		return res, err
	}
	for _, id := range res.Profiles {
		if _, err := tx.ExecContext(ctx, `INSERT INTO main.profiles (`+cols+`, serial) SELECT `+cols+
			`, (SELECT COALESCE(MAX(serial), 0) + 1 FROM main.profiles) FROM bak.profiles WHERE id = ?`, id); err != nil {
			return res, fmt.Errorf("merge profile %s: %w", id, err)
		}
		if err := copyRows("accounts", `profile_id = ?`, id); err != nil {
			return res, fmt.Errorf("merge accounts: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO main.profile_extensions (profile_id, extension_id)
			SELECT profile_id, extension_id FROM bak.profile_extensions
			WHERE profile_id = ? AND extension_id IN (SELECT id FROM main.extensions)`, id); err != nil {
			return res, fmt.Errorf("merge profile extensions: %w", err)
		}
		rows, err := tx.QueryContext(ctx, `SELECT secret_key, has_password, has_totp FROM bak.accounts WHERE profile_id = ?`, id)
		if err != nil {
			return res, err
		}
		for rows.Next() {
			var a MergedAccount
			if err := rows.Scan(&a.SecretKey, &a.Password, &a.TOTP); err != nil {
				rows.Close()
				return res, err
			}
			res.Accounts = append(res.Accounts, a)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return res, err
		}
	}
	if beforeCommit != nil {
		if err := beforeCommit(res); err != nil {
			return res, err
		}
	}
	return res, tx.Commit()
}

// AccountKeys lists every account's secret key and what it holds.
func (s *Store) AccountKeys() ([]MergedAccount, error) {
	rows, err := s.db.Query(`SELECT secret_key, has_password, has_totp FROM accounts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MergedAccount
	for rows.Next() {
		var a MergedAccount
		if err := rows.Scan(&a.SecretKey, &a.Password, &a.TOTP); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// sharedCols lists, comma separated, the columns table has in both
// databases, but those in except.
func sharedCols(ctx context.Context, tx *sql.Tx, table string, except ...string) (string, error) {
	mine, err := strs(ctx, tx, `SELECT name FROM pragma_table_info(?, 'main')`, table)
	if err != nil {
		return "", err
	}
	theirs, err := strs(ctx, tx, `SELECT name FROM pragma_table_info(?, 'bak')`, table)
	if err != nil {
		return "", err
	}
	in := map[string]bool{}
	for _, c := range theirs {
		in[c] = true
	}
	for _, c := range except {
		delete(in, c)
	}
	var cols []string
	for _, c := range mine {
		if in[c] {
			cols = append(cols, `"`+c+`"`)
		}
	}
	if len(cols) == 0 {
		return "", fmt.Errorf("table %s has no columns in common", table)
	}
	return strings.Join(cols, ","), nil
}

func strs(ctx context.Context, tx *sql.Tx, q string, args ...any) ([]string, error) {
	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

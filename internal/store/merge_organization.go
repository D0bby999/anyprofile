// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"context"
	"database/sql"
	"strings"
)

// Merge folders before their referring profiles/templates. Existing IDs win,
// and a same-name folder on the destination is reused rather than duplicated.
// The returned map translates source folder IDs to destination IDs.
func mergeOrganization(ctx context.Context, tx *sql.Tx) (map[string]string, []string, []string, error) {
	readFolders := func(schema string) ([]Folder, error) {
		rows, err := tx.QueryContext(ctx, `SELECT id, name, color, sort_order, created_at FROM `+schema+`.folders ORDER BY sort_order, created_at, id`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var fs []Folder
		for rows.Next() {
			var f Folder
			if err := rows.Scan(&f.ID, &f.Name, &f.Color, &f.SortOrder, &f.CreatedAt); err != nil {
				return nil, err
			}
			fs = append(fs, f)
		}
		return fs, rows.Err()
	}
	source, err := readFolders("bak")
	if err != nil {
		return nil, nil, nil, err
	}
	existing, err := readFolders("main")
	if err != nil {
		return nil, nil, nil, err
	}
	mapping := map[string]string{"": ""}
	var folders, templates []string
	for _, f := range source {
		target := ""
		for _, local := range existing {
			if local.ID == f.ID {
				target = local.ID
				break
			}
		}
		if target == "" {
			for _, local := range existing {
				if strings.EqualFold(strings.TrimSpace(local.Name), strings.TrimSpace(f.Name)) {
					target = local.ID
					break
				}
			}
		}
		if target == "" {
			if _, err := tx.ExecContext(ctx, `INSERT INTO main.folders (id,name,color,sort_order,created_at) VALUES (?,?,?,?,?)`, f.ID, f.Name, f.Color, f.SortOrder, f.CreatedAt); err != nil {
				return nil, nil, nil, err
			}
			existing = append(existing, f)
			folders = append(folders, f.ID)
			target = f.ID
		}
		mapping[f.ID] = target
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, folder_id FROM bak.templates WHERE id NOT IN (SELECT id FROM main.templates) ORDER BY rowid`)
	if err != nil {
		return nil, nil, nil, err
	}
	type templateFolder struct{ id, folder string }
	var pending []templateFolder
	for rows.Next() {
		var p templateFolder
		if err := rows.Scan(&p.id, &p.folder); err != nil {
			rows.Close()
			return nil, nil, nil, err
		}
		pending = append(pending, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, nil, err
	}
	cols, err := sharedCols(ctx, tx, "templates", "folder_id")
	if err != nil {
		return nil, nil, nil, err
	}
	for _, p := range pending {
		if _, err := tx.ExecContext(ctx, `INSERT INTO main.templates (`+cols+`,folder_id) SELECT `+cols+`,? FROM bak.templates WHERE id=?`, mapping[p.folder], p.id); err != nil {
			return nil, nil, nil, err
		}
		templates = append(templates, p.id)
	}
	return mapping, folders, templates, nil
}

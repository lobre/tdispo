package main

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mattn/go-sqlite3"
)

type Status struct {
	ID    int
	Label string
}

type StatusService struct {
	db *DB
}

func (s *StatusService) FindStatusByID(ctx context.Context, id int) (*Status, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	status, err := findStatusByID(ctx, tx, id)
	if err != nil {
		return nil, err
	}

	return status, nil
}

func (s *StatusService) FindStatuses(ctx context.Context) ([]*Status, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()

	return findStatuses(ctx, tx)
}

func (s *StatusService) CreateStatus(ctx context.Context, status *Status) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = createStatus(ctx, tx, status)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *StatusService) DeleteStatus(ctx context.Context, id int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = deleteStatus(ctx, tx, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func findStatuses(ctx context.Context, tx *sql.Tx) (_ []*Status, n int, err error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT 
			id,
			label,
			COUNT(*) OVER()
		FROM statuses
		ORDER BY label`,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	statuses := make([]*Status, 0)

	for rows.Next() {
		var s Status

		err = rows.Scan(&s.ID, &s.Label, &n)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, 0, ErrNoRecord
			}
			return nil, 0, err
		}

		statuses = append(statuses, &s)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return statuses, n, nil
}

func findStatusByID(ctx context.Context, tx *sql.Tx, id int) (*Status, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, label FROM statuses WHERE id = ?`, id)

	var status Status
	err := row.Scan(&status.ID, &status.Label)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoRecord
		}
		return nil, err
	}

	return &status, nil
}

func createStatus(ctx context.Context, tx *sql.Tx, status *Status) error {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO statuses (label) VALUES (?)`,
		status.Label,
	)
	if err != nil {
		return err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	status.ID = int(id)

	return nil
}

func deleteStatus(ctx context.Context, tx *sql.Tx, id int) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM statuses WHERE id = ?`, id)
	if err != nil {
		var sqliteError sqlite3.Error
		if errors.As(err, &sqliteError) {
			if sqliteError.ExtendedCode == sqlite3.ErrConstraintForeignKey {
				return ErrStatusUsed
			}
		}
		return err
	}

	return nil
}

package command

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/cloudradar-monitoring/rport/share/query"
	"github.com/cloudradar-monitoring/rport/share/random"
)

type SqliteProvider struct {
	db *sqlx.DB
}

var generateNewCommandID = func() (string, error) {
	return random.UUID4()
}

func NewSqliteProvider(db *sqlx.DB) *SqliteProvider {
	return &SqliteProvider{db: db}
}

func (p *SqliteProvider) Close() error {
	if p.db != nil {
		return p.db.Close()
	}

	return nil
}

func (p *SqliteProvider) GetByID(ctx context.Context, id string, ro *query.RetrieveOptions) (val *Command, found bool, err error) {
	q := "SELECT * FROM `commands` WHERE `id` = ? LIMIT 1"
	q = query.ConvertRetrieveOptionsToQuery(ro, q)

	val = new(Command)
	err = p.db.GetContext(ctx, val, q, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return val, false, nil
		}

		return val, false, err
	}

	return val, true, nil
}

func (p *SqliteProvider) List(ctx context.Context, lo *query.ListOptions) ([]Command, error) {
	values := []Command{}

	q := "SELECT * FROM `commands`"

	q, params := query.ConvertListOptionsToQuery(lo, q)

	err := p.db.SelectContext(ctx, &values, q, params...)
	if err != nil {
		return values, err
	}

	return values, nil
}

func (p *SqliteProvider) Save(ctx context.Context, s *Command) (string, error) {
	if s.ID == "" {
		commandID, err := generateNewCommandID()
		if err != nil {
			return commandID, err
		}

		_, err = p.db.ExecContext(
			ctx,
			"INSERT INTO `commands` (`id`, `name`, `created_at`, `created_by`, `updated_at`, `updated_by`, `cmd`) VALUES (?, ?, ?, ?, ?, ?, ?)",
			commandID,
			s.Name,
			s.CreatedAt.Format(time.RFC3339),
			s.CreatedBy,
			s.UpdatedAt.Format(time.RFC3339),
			s.UpdatedBy,
			s.Cmd,
		)

		return commandID, err
	}

	q := "UPDATE `commands` SET `name` = ?, `updated_at` = ?, `updated_by` = ?, `cmd` = ? WHERE id = ?"
	params := []interface{}{
		s.Name,
		s.UpdatedAt.Format(time.RFC3339),
		s.UpdatedBy,
		s.Cmd,
		s.ID,
	}
	_, err := p.db.ExecContext(ctx, q, params...)

	return s.ID, err
}

func (p *SqliteProvider) Delete(ctx context.Context, id string) error {
	res, err := p.db.ExecContext(ctx, "DELETE FROM `commands` WHERE `id` = ?", id)

	if err != nil {
		return err
	}

	affectedRows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if affectedRows == 0 {
		return fmt.Errorf("cannot find entry by id %s", id)
	}

	return nil
}

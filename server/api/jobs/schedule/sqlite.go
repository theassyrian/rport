package schedule

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/cloudradar-monitoring/rport/share/query"
)

const lastStartedAtFieldQuery = `(SELECT started_at FROM multi_jobs WHERE schedule_id = s.id ORDER BY started_at DESC LIMIT 1) AS last_started_at`

type SQLiteProvider struct {
	db *sqlx.DB
}

func newSQLiteProvider(db *sqlx.DB) *SQLiteProvider {
	return &SQLiteProvider{
		db: db,
	}
}

func (p *SQLiteProvider) Insert(ctx context.Context, s *Schedule) error {
	_, err := p.db.NamedExecContext(ctx,
		`INSERT INTO schedules (
			id,
			created_at,
			created_by,
			name,
			schedule,
			type,
			details
		) VALUES (
			:id,
			:created_at,
			:created_by,
			:name,
			:schedule,
			:type,
			:details
		)`,
		s.ToDB(),
	)

	return err
}

func (p *SQLiteProvider) Update(ctx context.Context, s *Schedule) error {
	_, err := p.db.NamedExecContext(ctx,
		`UPDATE schedules SET
			name = :name,
			schedule = :schedule,
			type = :type,
			details = :details
		WHERE id = :id`,
		s.ToDB(),
	)

	return err
}

func (p *SQLiteProvider) List(ctx context.Context, options *query.ListOptions) ([]*Schedule, error) {
	values := []*DBSchedule{}

	q := fmt.Sprintf("SELECT *, %s FROM `schedules` s", lastStartedAtFieldQuery)

	q, params := query.ConvertListOptionsToQuery(options, q)

	err := p.db.SelectContext(ctx, &values, q, params...)
	if err != nil {
		return nil, err
	}

	result := make([]*Schedule, len(values))
	for i, v := range values {
		result[i] = v.ToSchedule()
	}

	return result, nil
}

func (p *SQLiteProvider) Close() error {
	return p.db.Close()
}

func (p *SQLiteProvider) Get(ctx context.Context, id string) (*Schedule, error) {
	q := fmt.Sprintf(
		"SELECT *, %s FROM `schedules` s WHERE `id` = ? LIMIT 1",
		lastStartedAtFieldQuery,
	)

	s := &DBSchedule{}
	err := p.db.GetContext(ctx, s, q, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return s.ToSchedule(), nil
}

func (p *SQLiteProvider) Delete(ctx context.Context, id string) error {
	res, err := p.db.ExecContext(ctx, "DELETE FROM `schedules` WHERE `id` = ?", id)

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

	// Delete associated jobs
	_, err = p.db.ExecContext(ctx, "DELETE FROM jobs WHERE multi_job_id IN (SELECT jid FROM multi_jobs WHERE schedule_id = ?)", id)
	if err != nil {
		return err
	}

	// Delete associated multi jobs
	_, err = p.db.ExecContext(ctx, "DELETE FROM multi_jobs WHERE schedule_id = ?", id)
	if err != nil {
		return err
	}

	return nil
}

// CountJobsInProgress counts jobs for scheduleID that have not finished and are not timed out
func (p *SQLiteProvider) CountJobsInProgress(ctx context.Context, scheduleID string, timeoutSec int) (int, error) {
	var result int

	err := p.db.GetContext(ctx, &result, `
SELECT count(*)
FROM jobs
JOIN multi_jobs ON jobs.multi_job_id = multi_jobs.jid
WHERE
	schedule_id = ?
AND
	finished_at IS NULL
AND
	strftime('%s', 'now') - strftime('%s', jobs.started_at) <= ?
`, scheduleID, timeoutSec)
	if err != nil {
		return 0, err
	}

	return result, nil
}

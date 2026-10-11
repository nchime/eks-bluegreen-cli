package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	apperrors "eks-bluegreen-cli/pkg/errors"
	"eks-bluegreen-cli/pkg/models"

	_ "modernc.org/sqlite"
)

type sqliteStore struct {
	db *sql.DB
}

// NewSQLiteStore initializes and connects to SQLite store.
func NewSQLiteStore(dbPath string) (Store, error) {
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %q: %w", dbPath, err)
	}

	// Performance and reliability pragmas
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;"); err != nil {
		// WAL might not be supported in-memory, ignore if fail
	}

	s := &sqliteStore{db: db}
	if err := s.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return s, nil
}

func (s *sqliteStore) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS upgrade_jobs (
			id TEXT PRIMARY KEY,
			source_cluster TEXT NOT NULL,
			source_region TEXT NOT NULL,
			target_cluster TEXT NOT NULL,
			target_region TEXT NOT NULL,
			source_version TEXT NOT NULL,
			target_version TEXT NOT NULL,
			current_phase TEXT NOT NULL,
			status TEXT NOT NULL,
			plan_path TEXT,
			plan_hash TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			last_error TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS resource_plans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			resource_name TEXT NOT NULL,
			action TEXT NOT NULL,
			dependencies TEXT NOT NULL,
			risk_level TEXT NOT NULL,
			validation_status TEXT NOT NULL,
			notes TEXT,
			FOREIGN KEY(job_id) REFERENCES upgrade_jobs(id)
		);`,
		`CREATE TABLE IF NOT EXISTS approval_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT NOT NULL,
			approval_type TEXT NOT NULL,
			approver TEXT NOT NULL,
			approved_at DATETIME NOT NULL,
			plan_hash TEXT NOT NULL,
			validation_hash TEXT NOT NULL,
			status TEXT NOT NULL,
			comments TEXT,
			FOREIGN KEY(job_id) REFERENCES upgrade_jobs(id)
		);`,
		`CREATE TABLE IF NOT EXISTS execution_steps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT NOT NULL,
			step_name TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at DATETIME NOT NULL,
			completed_at DATETIME,
			retry_count INTEGER NOT NULL DEFAULT 0,
			error_message TEXT,
			output_reference TEXT,
			FOREIGN KEY(job_id) REFERENCES upgrade_jobs(id)
		);`,
		`CREATE TABLE IF NOT EXISTS validation_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT NOT NULL,
			check_name TEXT NOT NULL,
			severity TEXT NOT NULL,
			status TEXT NOT NULL,
			details TEXT NOT NULL,
			checked_at DATETIME NOT NULL,
			FOREIGN KEY(job_id) REFERENCES upgrade_jobs(id)
		);`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (s *sqliteStore) CreateJob(ctx context.Context, job *models.UpgradeJob) error {
	now := time.Now().UTC()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now

	query := `INSERT INTO upgrade_jobs (
		id, source_cluster, source_region, target_cluster, target_region,
		source_version, target_version, current_phase, status,
		plan_path, plan_hash, created_at, updated_at, last_error
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

	_, err := s.db.ExecContext(ctx, query,
		job.ID, job.SourceCluster, job.SourceRegion, job.TargetCluster, job.TargetRegion,
		job.SourceVersion, job.TargetVersion, string(job.CurrentPhase), string(job.Status),
		job.PlanPath, job.PlanHash, job.CreatedAt, job.UpdatedAt, job.LastError,
	)
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}
	return nil
}

func (s *sqliteStore) GetJob(ctx context.Context, id string) (*models.UpgradeJob, error) {
	query := `SELECT id, source_cluster, source_region, target_cluster, target_region,
		source_version, target_version, current_phase, status,
		plan_path, plan_hash, created_at, updated_at, last_error
		FROM upgrade_jobs WHERE id = ?;`

	row := s.db.QueryRowContext(ctx, query, id)
	var job models.UpgradeJob
	var planPath, planHash, lastErr sql.NullString
	var phase, status string

	err := row.Scan(
		&job.ID, &job.SourceCluster, &job.SourceRegion, &job.TargetCluster, &job.TargetRegion,
		&job.SourceVersion, &job.TargetVersion, &phase, &status,
		&planPath, &planHash, &job.CreatedAt, &job.UpdatedAt, &lastErr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.ErrJobNotFound
		}
		return nil, fmt.Errorf("failed to get job %q: %w", id, err)
	}

	job.CurrentPhase = models.JobPhase(phase)
	job.Status = models.JobStatus(status)
	if planPath.Valid {
		job.PlanPath = planPath.String
	}
	if planHash.Valid {
		job.PlanHash = planHash.String
	}
	if lastErr.Valid {
		job.LastError = lastErr.String
	}

	return &job, nil
}

func (s *sqliteStore) UpdateJob(ctx context.Context, job *models.UpgradeJob) error {
	job.UpdatedAt = time.Now().UTC()
	query := `UPDATE upgrade_jobs SET
		current_phase = ?, status = ?, plan_path = ?, plan_hash = ?,
		updated_at = ?, last_error = ?
		WHERE id = ?;`

	res, err := s.db.ExecContext(ctx, query,
		string(job.CurrentPhase), string(job.Status), job.PlanPath, job.PlanHash,
		job.UpdatedAt, job.LastError, job.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update job %q: %w", job.ID, err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return apperrors.ErrJobNotFound
	}
	return nil
}

func (s *sqliteStore) ListJobs(ctx context.Context) ([]*models.UpgradeJob, error) {
	query := `SELECT id, source_cluster, source_region, target_cluster, target_region,
		source_version, target_version, current_phase, status,
		plan_path, plan_hash, created_at, updated_at, last_error
		FROM upgrade_jobs ORDER BY created_at DESC;`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*models.UpgradeJob
	for rows.Next() {
		var job models.UpgradeJob
		var planPath, planHash, lastErr sql.NullString
		var phase, status string

		if err := rows.Scan(
			&job.ID, &job.SourceCluster, &job.SourceRegion, &job.TargetCluster, &job.TargetRegion,
			&job.SourceVersion, &job.TargetVersion, &phase, &status,
			&planPath, &planHash, &job.CreatedAt, &job.UpdatedAt, &lastErr,
		); err != nil {
			return nil, err
		}

		job.CurrentPhase = models.JobPhase(phase)
		job.Status = models.JobStatus(status)
		if planPath.Valid {
			job.PlanPath = planPath.String
		}
		if planHash.Valid {
			job.PlanHash = planHash.String
		}
		if lastErr.Valid {
			job.LastError = lastErr.String
		}
		jobs = append(jobs, &job)
	}
	return jobs, nil
}

func (s *sqliteStore) SaveResourcePlans(ctx context.Context, jobID string, plans []models.ResourcePlan) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM resource_plans WHERE job_id = ?", jobID); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO resource_plans (
		job_id, resource_type, resource_name, action, dependencies, risk_level, validation_status, notes
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range plans {
		depsJSON, err := json.Marshal(p.Dependencies)
		if err != nil {
			depsJSON = []byte("[]")
		}
		if _, err := stmt.ExecContext(ctx,
			jobID, p.ResourceType, p.ResourceName, string(p.Action), string(depsJSON),
			string(p.RiskLevel), p.ValidationStatus, p.Notes,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *sqliteStore) GetResourcePlans(ctx context.Context, jobID string) ([]models.ResourcePlan, error) {
	query := `SELECT resource_type, resource_name, action, dependencies, risk_level, validation_status, notes
		FROM resource_plans WHERE job_id = ? ORDER BY id ASC`

	rows, err := s.db.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plans []models.ResourcePlan
	for rows.Next() {
		var p models.ResourcePlan
		var action, risk, depsStr string
		p.JobID = jobID

		if err := rows.Scan(
			&p.ResourceType, &p.ResourceName, &action, &depsStr, &risk, &p.ValidationStatus, &p.Notes,
		); err != nil {
			return nil, err
		}
		p.Action = models.ActionType(action)
		p.RiskLevel = models.RiskLevel(risk)
		_ = json.Unmarshal([]byte(depsStr), &p.Dependencies)
		plans = append(plans, p)
	}
	return plans, nil
}

func (s *sqliteStore) SaveApproval(ctx context.Context, approval *models.ApprovalRecord) error {
	if approval.ApprovedAt.IsZero() {
		approval.ApprovedAt = time.Now().UTC()
	}

	query := `INSERT INTO approval_records (
		job_id, approval_type, approver, approved_at, plan_hash, validation_hash, status, comments
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		approval.JobID, approval.ApprovalType, approval.Approver, approval.ApprovedAt,
		approval.PlanHash, approval.ValidationHash, string(approval.Status), approval.Comments,
	)
	return err
}

func (s *sqliteStore) GetLatestApproval(ctx context.Context, jobID string) (*models.ApprovalRecord, error) {
	query := `SELECT job_id, approval_type, approver, approved_at, plan_hash, validation_hash, status, comments
		FROM approval_records WHERE job_id = ? ORDER BY approved_at DESC LIMIT 1`

	row := s.db.QueryRowContext(ctx, query, jobID)
	var rec models.ApprovalRecord
	var status string
	var comments sql.NullString

	if err := row.Scan(
		&rec.JobID, &rec.ApprovalType, &rec.Approver, &rec.ApprovedAt,
		&rec.PlanHash, &rec.ValidationHash, &status, &comments,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No approval yet
		}
		return nil, err
	}

	rec.Status = models.ApprovalStatus(status)
	if comments.Valid {
		rec.Comments = comments.String
	}
	return &rec, nil
}

func (s *sqliteStore) SaveExecutionStep(ctx context.Context, step *models.ExecutionStep) error {
	if step.StartedAt.IsZero() {
		step.StartedAt = time.Now().UTC()
	}

	query := `INSERT INTO execution_steps (
		job_id, step_name, status, started_at, completed_at, retry_count, error_message, output_reference
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	res, err := s.db.ExecContext(ctx, query,
		step.JobID, step.StepName, string(step.Status), step.StartedAt, step.CompletedAt,
		step.RetryCount, step.ErrorMessage, step.OutputReference,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		step.ID = id
	}
	return nil
}

func (s *sqliteStore) GetExecutionSteps(ctx context.Context, jobID string) ([]models.ExecutionStep, error) {
	query := `SELECT id, job_id, step_name, status, started_at, completed_at, retry_count, error_message, output_reference
		FROM execution_steps WHERE job_id = ? ORDER BY id ASC`

	rows, err := s.db.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []models.ExecutionStep
	for rows.Next() {
		var st models.ExecutionStep
		var status string
		var completedAt sql.NullTime
		var errMsg, outputRef sql.NullString

		if err := rows.Scan(
			&st.ID, &st.JobID, &st.StepName, &status, &st.StartedAt, &completedAt,
			&st.RetryCount, &errMsg, &outputRef,
		); err != nil {
			return nil, err
		}

		st.Status = models.StepStatus(status)
		if completedAt.Valid {
			t := completedAt.Time
			st.CompletedAt = &t
		}
		if errMsg.Valid {
			st.ErrorMessage = errMsg.String
		}
		if outputRef.Valid {
			st.OutputReference = outputRef.String
		}
		steps = append(steps, st)
	}
	return steps, nil
}

func (s *sqliteStore) SaveValidationResults(ctx context.Context, jobID string, results []models.ValidationResult) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM validation_results WHERE job_id = ?", jobID); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO validation_results (
		job_id, check_name, severity, status, details, checked_at
	) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range results {
		checkedAt := r.CheckedAt
		if checkedAt.IsZero() {
			checkedAt = time.Now().UTC()
		}
		if _, err := stmt.ExecContext(ctx,
			jobID, r.CheckName, string(r.Severity), string(r.Status), r.Details, checkedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *sqliteStore) GetValidationResults(ctx context.Context, jobID string) ([]models.ValidationResult, error) {
	query := `SELECT job_id, check_name, severity, status, details, checked_at
		FROM validation_results WHERE job_id = ? ORDER BY id ASC`

	rows, err := s.db.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.ValidationResult
	for rows.Next() {
		var r models.ValidationResult
		var severity, status string

		if err := rows.Scan(&r.JobID, &r.CheckName, &severity, &status, &r.Details, &r.CheckedAt); err != nil {
			return nil, err
		}
		r.Severity = models.Severity(severity)
		r.Status = models.ValidationStatus(status)
		results = append(results, r)
	}
	return results, nil
}

func (s *sqliteStore) Close() error {
	return s.db.Close()
}

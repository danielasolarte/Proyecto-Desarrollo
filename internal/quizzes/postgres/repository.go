package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/equipo-mooc/plataforma-mooc/internal/quizzes/domain"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) CreateQuiz(ctx context.Context, quiz *domain.Quiz) error {
	query := `
		INSERT INTO quizzes (
			id,
			resource_id,
			passing_score,
			max_attempts,
			time_limit_seconds,
			feedback_mode,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, now(), now())
		RETURNING created_at, updated_at
	`

	if quiz.ID == uuid.Nil {
		quiz.ID = uuid.New()
	}

	err := r.db.QueryRow(
		ctx,
		query,
		quiz.ID,
		quiz.ResourceID,
		quiz.PassingScore,
		quiz.MaxAttempts,
		quiz.TimeLimitSeconds,
		quiz.FeedbackMode,
	).Scan(
		&quiz.CreatedAt,
		&quiz.UpdatedAt,
	)

	return err
}

func (r *Repository) GetQuizByID(ctx context.Context, id uuid.UUID) (*domain.Quiz, error) {
	query := `
		SELECT
			id,
			resource_id,
			passing_score,
			max_attempts,
			time_limit_seconds,
			feedback_mode,
			created_at,
			updated_at
		FROM quizzes
		WHERE id = $1
	`

	var quiz domain.Quiz

	err := r.db.QueryRow(ctx, query, id).Scan(
		&quiz.ID,
		&quiz.ResourceID,
		&quiz.PassingScore,
		&quiz.MaxAttempts,
		&quiz.TimeLimitSeconds,
		&quiz.FeedbackMode,
		&quiz.CreatedAt,
		&quiz.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrQuizNotFound
	}

	if err != nil {
		return nil, err
	}

	return &quiz, nil
}

func (r *Repository) UpdateQuiz(ctx context.Context, quiz *domain.Quiz) error {
	query := `
		UPDATE quizzes
		SET
			passing_score = $2,
			max_attempts = $3,
			time_limit_seconds = $4,
			feedback_mode = $5,
			updated_at = now()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		quiz.ID,
		quiz.PassingScore,
		quiz.MaxAttempts,
		quiz.TimeLimitSeconds,
		quiz.FeedbackMode,
	).Scan(&quiz.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrQuizNotFound
	}

	return err
}

func (r *Repository) DeleteQuiz(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Exec(
		ctx,
		`DELETE FROM quizzes WHERE id = $1`,
		id,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrQuizNotFound
	}

	return nil
}

func (r *Repository) CreateQuestion(
	ctx context.Context,
	question *domain.Question,
) error {
	query := `
		INSERT INTO questions (
			id,
			quiz_id,
			stable_id,
			text,
			position,
			points,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, now(), now())
		RETURNING created_at, updated_at
	`

	if question.ID == uuid.Nil {
		question.ID = uuid.New()
	}

	if question.StableID == uuid.Nil {
		question.StableID = uuid.New()
	}

	err := r.db.QueryRow(
		ctx,
		query,
		question.ID,
		question.QuizID,
		question.StableID,
		question.Text,
		question.Position,
		question.Points,
	).Scan(
		&question.CreatedAt,
		&question.UpdatedAt,
	)

	return err
}

func (r *Repository) GetQuestionByID(
	ctx context.Context,
	id uuid.UUID,
) (*domain.Question, error) {
	query := `
		SELECT
			id,
			quiz_id,
			stable_id,
			text,
			position,
			points,
			created_at,
			updated_at
		FROM questions
		WHERE id = $1
	`

	var question domain.Question

	err := r.db.QueryRow(ctx, query, id).Scan(
		&question.ID,
		&question.QuizID,
		&question.StableID,
		&question.Text,
		&question.Position,
		&question.Points,
		&question.CreatedAt,
		&question.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrQuestionNotFound
	}

	if err != nil {
		return nil, err
	}

	return &question, nil
}

func (r *Repository) UpdateQuestion(
	ctx context.Context,
	question *domain.Question,
) error {
	query := `
		UPDATE questions
		SET
			text = $2,
			position = $3,
			points = $4,
			updated_at = now()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		question.ID,
		question.Text,
		question.Position,
		question.Points,
	).Scan(&question.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrQuestionNotFound
	}

	return err
}

func (r *Repository) DeleteQuestion(
	ctx context.Context,
	id uuid.UUID,
) error {
	result, err := r.db.Exec(
		ctx,
		`DELETE FROM questions WHERE id = $1`,
		id,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrQuestionNotFound
	}

	return nil
}

func (r *Repository) CreateOption(
	ctx context.Context,
	option *domain.QuestionOption,
) error {
	query := `
		INSERT INTO question_options (
			id,
			question_id,
			text,
			position,
			is_correct,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, now(), now())
		RETURNING created_at, updated_at
	`

	if option.ID == uuid.Nil {
		option.ID = uuid.New()
	}

	err := r.db.QueryRow(
		ctx,
		query,
		option.ID,
		option.QuestionID,
		option.Text,
		option.Position,
		option.IsCorrect,
	).Scan(
		&option.CreatedAt,
		&option.UpdatedAt,
	)

	return err
}

func (r *Repository) UpdateOption(
	ctx context.Context,
	option *domain.QuestionOption,
) error {
	query := `
		UPDATE question_options
		SET
			text = $2,
			position = $3,
			is_correct = $4,
			updated_at = now()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		option.ID,
		option.Text,
		option.Position,
		option.IsCorrect,
	).Scan(&option.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrQuestionNotFound
	}

	return err
}

func (r *Repository) DeleteOption(
	ctx context.Context,
	id uuid.UUID,
) error {
	result, err := r.db.Exec(
		ctx,
		`DELETE FROM question_options WHERE id = $1`,
		id,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrQuestionNotFound
	}

	return nil
}

func (r *Repository) GetQuestionsByQuizID(
	ctx context.Context,
	quizID uuid.UUID,
) ([]domain.Question, error) {
	rows, err := r.db.Query(
		ctx,
		`
		SELECT
			id,
			quiz_id,
			stable_id,
			text,
			position,
			points,
			created_at,
			updated_at
		FROM questions
		WHERE quiz_id = $1
		ORDER BY position
		`,
		quizID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []domain.Question

	for rows.Next() {
		var question domain.Question

		err := rows.Scan(
			&question.ID,
			&question.QuizID,
			&question.StableID,
			&question.Text,
			&question.Position,
			&question.Points,
			&question.CreatedAt,
			&question.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		options, err := r.getOptionsByQuestionID(ctx, question.ID)
		if err != nil {
			return nil, err
		}

		question.Options = options
		questions = append(questions, question)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return questions, nil
}

func (r *Repository) getOptionsByQuestionID(
	ctx context.Context,
	questionID uuid.UUID,
) ([]domain.QuestionOption, error) {
	rows, err := r.db.Query(
		ctx,
		`
		SELECT
			id,
			question_id,
			text,
			position,
			is_correct,
			created_at,
			updated_at
		FROM question_options
		WHERE question_id = $1
		ORDER BY position
		`,
		questionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var options []domain.QuestionOption

	for rows.Next() {
		var option domain.QuestionOption

		err := rows.Scan(
			&option.ID,
			&option.QuestionID,
			&option.Text,
			&option.Position,
			&option.IsCorrect,
			&option.CreatedAt,
			&option.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		options = append(options, option)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return options, nil
}

func (r *Repository) GetAttemptByID(
	ctx context.Context,
	id uuid.UUID,
) (*domain.Attempt, error) {
	query := `
		SELECT
			id,
			quiz_id,
			student_id,
			enrollment_id,
			status,
			score,
			percentage,
			passed,
			snapshot,
			idempotency_key,
			started_at,
			expires_at,
			submitted_at,
			created_at,
			updated_at
		FROM attempts
		WHERE id = $1
	`

	var attempt domain.Attempt

	err := r.db.QueryRow(ctx, query, id).Scan(
		&attempt.ID,
		&attempt.QuizID,
		&attempt.StudentID,
		&attempt.EnrollmentID,
		&attempt.Status,
		&attempt.Score,
		&attempt.Percentage,
		&attempt.Passed,
		&attempt.Snapshot,
		&attempt.IdempotencyKey,
		&attempt.StartedAt,
		&attempt.ExpiresAt,
		&attempt.SubmittedAt,
		&attempt.CreatedAt,
		&attempt.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAttemptNotFound
	}

	if err != nil {
		return nil, err
	}

	return &attempt, nil
}

func (r *Repository) CountAttemptsByStudent(
	ctx context.Context,
	quizID uuid.UUID,
	studentID uuid.UUID,
) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM attempts
		WHERE quiz_id = $1
		  AND student_id = $2
	`

	var count int

	err := r.db.QueryRow(
		ctx,
		query,
		quizID,
		studentID,
	).Scan(&count)

	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *Repository) SaveAnswer(
	ctx context.Context,
	answer *domain.AttemptAnswer,
) error {
	query := `
		INSERT INTO attempt_answers (
			id,
			attempt_id,
			question_id,
			selected_option_id,
			saved_at,
			created_at,
			updated_at
		)
		VALUES (
			$1, $2, $3, $4,
			now(), now(), now()
		)
		ON CONFLICT (attempt_id, question_id)
		DO UPDATE SET
			selected_option_id = EXCLUDED.selected_option_id,
			saved_at = now(),
			updated_at = now()
		RETURNING
			saved_at,
			created_at,
			updated_at
	`

	if answer.ID == uuid.Nil {
		answer.ID = uuid.New()
	}

	err := r.db.QueryRow(
		ctx,
		query,
		answer.ID,
		answer.AttemptID,
		answer.QuestionID,
		answer.SelectedOptionID,
	).Scan(
		&answer.SavedAt,
		&answer.CreatedAt,
		&answer.UpdatedAt,
	)

	return err
}

func (r *Repository) GetAnswersByAttemptID(
	ctx context.Context,
	attemptID uuid.UUID,
) ([]domain.AttemptAnswer, error) {
	rows, err := r.db.Query(
		ctx,
		`
		SELECT
			id,
			attempt_id,
			question_id,
			selected_option_id,
			saved_at,
			created_at,
			updated_at
		FROM attempt_answers
		WHERE attempt_id = $1
		ORDER BY saved_at
		`,
		attemptID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var answers []domain.AttemptAnswer

	for rows.Next() {
		var answer domain.AttemptAnswer

		err := rows.Scan(
			&answer.ID,
			&answer.AttemptID,
			&answer.QuestionID,
			&answer.SelectedOptionID,
			&answer.SavedAt,
			&answer.CreatedAt,
			&answer.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		answers = append(answers, answer)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return answers, nil
}

func (r *Repository) CreateAttempt(
	ctx context.Context,
	attempt *domain.Attempt,
) error {
	query := `
		INSERT INTO attempts (
			id,
			quiz_id,
			student_id,
			enrollment_id,
			status,
			score,
			percentage,
			passed,
			snapshot,
			idempotency_key,
			started_at,
			expires_at,
			submitted_at,
			created_at,
			updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, now(), now()
		)
		RETURNING created_at, updated_at
	`

	if attempt.ID == uuid.Nil {
		attempt.ID = uuid.New()
	}

	if attempt.StartedAt.IsZero() {
		attempt.StartedAt = time.Now()
	}

	err := r.db.QueryRow(
		ctx,
		query,
		attempt.ID,
		attempt.QuizID,
		attempt.StudentID,
		attempt.EnrollmentID,
		attempt.Status,
		attempt.Score,
		attempt.Percentage,
		attempt.Passed,
		attempt.Snapshot,
		attempt.IdempotencyKey,
		attempt.StartedAt,
		attempt.ExpiresAt,
		attempt.SubmittedAt,
	).Scan(
		&attempt.CreatedAt,
		&attempt.UpdatedAt,
	)

	return err
}

func (r *Repository) GetOptionByID(
	ctx context.Context,
	id uuid.UUID,
) (*domain.QuestionOption, error) {
	query := `
		SELECT
			id,
			question_id,
			text,
			position,
			is_correct,
			created_at,
			updated_at
		FROM question_options
		WHERE id = $1
	`

	var option domain.QuestionOption

	err := r.db.QueryRow(ctx, query, id).Scan(
		&option.ID,
		&option.QuestionID,
		&option.Text,
		&option.Position,
		&option.IsCorrect,
		&option.CreatedAt,
		&option.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrOptionNotFound
	}

	if err != nil {
		return nil, err
	}

	return &option, nil
}

func (r *Repository) GetAttemptByIdempotencyKey(
	ctx context.Context,
	studentID uuid.UUID,
	key string,
) (*domain.Attempt, error) {
	query := `
		SELECT
			id,
			quiz_id,
			student_id,
			enrollment_id,
			status,
			score,
			percentage,
			passed,
			snapshot,
			idempotency_key,
			started_at,
			expires_at,
			submitted_at,
			created_at,
			updated_at
		FROM attempts
		WHERE student_id = $1
		  AND idempotency_key = $2
	`

	var attempt domain.Attempt

	err := r.db.QueryRow(ctx, query, studentID, key).Scan(
		&attempt.ID,
		&attempt.QuizID,
		&attempt.StudentID,
		&attempt.EnrollmentID,
		&attempt.Status,
		&attempt.Score,
		&attempt.Percentage,
		&attempt.Passed,
		&attempt.Snapshot,
		&attempt.IdempotencyKey,
		&attempt.StartedAt,
		&attempt.ExpiresAt,
		&attempt.SubmittedAt,
		&attempt.CreatedAt,
		&attempt.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAttemptNotFound
	}

	if err != nil {
		return nil, err
	}

	return &attempt, nil
}

func (r *Repository) GetQuizByResourceID(
	ctx context.Context,
	resourceID uuid.UUID,
) (*domain.Quiz, error) {
	query := `
		SELECT
			id,
			resource_id,
			passing_score,
			max_attempts,
			time_limit_seconds,
			feedback_mode,
			created_at,
			updated_at
		FROM quizzes
		WHERE resource_id = $1
	`

	var quiz domain.Quiz

	err := r.db.QueryRow(
		ctx,
		query,
		resourceID,
	).Scan(
		&quiz.ID,
		&quiz.ResourceID,
		&quiz.PassingScore,
		&quiz.MaxAttempts,
		&quiz.TimeLimitSeconds,
		&quiz.FeedbackMode,
		&quiz.CreatedAt,
		&quiz.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrQuizNotFound
	}

	if err != nil {
		return nil, err
	}

	return &quiz, nil
}

func (r *Repository) UpdateAttempt(
	ctx context.Context,
	attempt *domain.Attempt,
) error {
	query := `
		UPDATE attempts
		SET
			status = $2,
			score = $3,
			percentage = $4,
			passed = $5,
			snapshot = $6,
			idempotency_key = $7,
			expires_at = $8,
			submitted_at = $9,
			updated_at = now()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		attempt.ID,
		attempt.Status,
		attempt.Score,
		attempt.Percentage,
		attempt.Passed,
		attempt.Snapshot,
		attempt.IdempotencyKey,
		attempt.ExpiresAt,
		attempt.SubmittedAt,
	).Scan(
		&attempt.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrAttemptNotFound
	}

	return err
}




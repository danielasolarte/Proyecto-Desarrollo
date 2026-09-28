CREATE TABLE quizzes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    resource_id UUID NOT NULL
        REFERENCES resources(id) ON DELETE CASCADE,

    passing_score NUMERIC(5,2) NOT NULL DEFAULT 60
        CHECK (passing_score >= 0 AND passing_score <= 100),

    max_attempts INT
        CHECK (max_attempts IS NULL OR max_attempts > 0),

    time_limit_seconds INT
        CHECK (time_limit_seconds IS NULL OR time_limit_seconds > 0),

    feedback_mode TEXT NOT NULL DEFAULT 'after_submit'
        CHECK (feedback_mode IN (
            'none',
            'after_submit',
            'after_pass'
        )),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (resource_id)
);

CREATE TABLE questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    quiz_id UUID NOT NULL
        REFERENCES quizzes(id) ON DELETE CASCADE,

    stable_id UUID NOT NULL DEFAULT gen_random_uuid(),

    text TEXT NOT NULL,

    position INT NOT NULL
        CHECK (position > 0),

    points NUMERIC(8,2) NOT NULL DEFAULT 1
        CHECK (points > 0),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (quiz_id, position),
    UNIQUE (quiz_id, stable_id)
);

CREATE TABLE question_options (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    question_id UUID NOT NULL
        REFERENCES questions(id) ON DELETE CASCADE,

    text TEXT NOT NULL,

    position INT NOT NULL
        CHECK (position > 0),

    is_correct BOOLEAN NOT NULL DEFAULT false,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (question_id, position)
);

CREATE TABLE attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    quiz_id UUID NOT NULL
        REFERENCES quizzes(id) ON DELETE CASCADE,

    student_id UUID NOT NULL
        REFERENCES users(id) ON DELETE CASCADE,

    enrollment_id UUID NOT NULL
        REFERENCES enrollments(id) ON DELETE CASCADE,

    status TEXT NOT NULL DEFAULT 'in_progress'
        CHECK (status IN (
            'in_progress',
            'submitted',
            'expired'
        )),

    score NUMERIC(8,2),

    percentage NUMERIC(5,2)
        CHECK (
            percentage IS NULL
            OR (percentage >= 0 AND percentage <= 100)
        ),

    passed BOOLEAN,

    snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,

    idempotency_key TEXT,

    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    expires_at TIMESTAMPTZ,

    submitted_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE attempt_answers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    attempt_id UUID NOT NULL
        REFERENCES attempts(id) ON DELETE CASCADE,

    question_id UUID NOT NULL
        REFERENCES questions(id) ON DELETE CASCADE,

    selected_option_id UUID
        REFERENCES question_options(id) ON DELETE SET NULL,

    saved_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (attempt_id, question_id)
);

CREATE TABLE resource_progress (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    student_id UUID NOT NULL
        REFERENCES users(id) ON DELETE CASCADE,

    enrollment_id UUID NOT NULL
        REFERENCES enrollments(id) ON DELETE CASCADE,

    resource_id UUID NOT NULL
        REFERENCES resources(id) ON DELETE CASCADE,

    resource_stable_id UUID NOT NULL,

    status TEXT NOT NULL DEFAULT 'not_started'
        CHECK (status IN (
            'not_started',
            'in_progress',
            'completed'
        )),

    opened_at TIMESTAMPTZ,

    last_activity_at TIMESTAMPTZ,

    completed_at TIMESTAMPTZ,

    active_seconds INT NOT NULL DEFAULT 0
        CHECK (active_seconds >= 0),

    last_position_seconds INT
        CHECK (
            last_position_seconds IS NULL
            OR last_position_seconds >= 0
        ),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (student_id, enrollment_id, resource_stable_id)
);

CREATE TABLE progress_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    student_id UUID NOT NULL
        REFERENCES users(id) ON DELETE CASCADE,

    enrollment_id UUID NOT NULL
        REFERENCES enrollments(id) ON DELETE CASCADE,

    resource_id UUID NOT NULL
        REFERENCES resources(id) ON DELETE CASCADE,

    resource_stable_id UUID NOT NULL,

    event_type TEXT NOT NULL
        CHECK (event_type IN (
            'opened',
            'heartbeat',
            'position',
            'completed',
            'quiz_submitted',
            'quiz_passed'
        )),

    position_seconds INT
        CHECK (
            position_seconds IS NULL
            OR position_seconds >= 0
        ),

    client_timestamp TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE course_progress (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    student_id UUID NOT NULL
        REFERENCES users(id) ON DELETE CASCADE,

    enrollment_id UUID NOT NULL
        REFERENCES enrollments(id) ON DELETE CASCADE,

    course_id UUID NOT NULL
        REFERENCES courses(id) ON DELETE CASCADE,

    status TEXT NOT NULL DEFAULT 'in_progress'
        CHECK (status IN (
            'in_progress',
            'completed',
            'approved'
        )),

    progress_percentage NUMERIC(5,2) NOT NULL DEFAULT 0
        CHECK (
            progress_percentage >= 0
            AND progress_percentage <= 100
        ),

    completed_at TIMESTAMPTZ,

    approved_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (student_id, enrollment_id, course_id)
);

CREATE TABLE badges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    course_id UUID NOT NULL
        REFERENCES courses(id) ON DELETE CASCADE,

    name TEXT NOT NULL,

    description TEXT,

    image_url TEXT,

    active BOOLEAN NOT NULL DEFAULT true,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (course_id)
);

CREATE TABLE badge_issuances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    badge_id UUID NOT NULL
        REFERENCES badges(id) ON DELETE CASCADE,

    student_id UUID NOT NULL
        REFERENCES users(id) ON DELETE CASCADE,

    course_id UUID NOT NULL
        REFERENCES courses(id) ON DELETE CASCADE,

    enrollment_id UUID NOT NULL
        REFERENCES enrollments(id) ON DELETE CASCADE,

    verification_code UUID NOT NULL DEFAULT gen_random_uuid(),

    issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    revoked_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (student_id, course_id, badge_id),
    UNIQUE (verification_code)
);

CREATE INDEX idx_questions_quiz_id ON questions(quiz_id);

CREATE INDEX idx_question_options_question_id ON question_options(question_id);

CREATE INDEX idx_attempts_quiz_student ON attempts(quiz_id, student_id);

CREATE INDEX idx_attempts_enrollment ON attempts(enrollment_id);

CREATE INDEX idx_attempt_answers_attempt ON attempt_answers(attempt_id);

CREATE INDEX idx_resource_progress_student ON resource_progress(student_id);

CREATE INDEX idx_resource_progress_enrollment ON resource_progress(enrollment_id);

CREATE INDEX idx_resource_progress_stable ON resource_progress(resource_stable_id);

CREATE INDEX idx_progress_events_student_resource ON progress_events(student_id, resource_stable_id);

CREATE INDEX idx_course_progress_student ON course_progress(student_id);

CREATE INDEX idx_course_progress_course ON course_progress(course_id);

CREATE INDEX idx_badge_issuances_student ON badge_issuances(student_id);

CREATE INDEX idx_badge_issuances_verification ON badge_issuances(verification_code);

CREATE UNIQUE INDEX idx_attempts_student_idempotency_key ON attempts(student_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
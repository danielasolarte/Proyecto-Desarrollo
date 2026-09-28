-- Módulo: Cursos, Autoría y Catálogo (Persona B / Persona 1)
--
-- IMPORTANTE PARA EL EQUIPO: golang-migrate exige que los números de
-- secuencia (000001, 000002, ...) sean únicos y consecutivos en TODO el
-- repositorio, no por módulo. Si otra persona ya creó una migración
-- 000001_algo.up.sql, hay que renombrar este archivo (y su .down.sql) al
-- siguiente número libre antes de hacer merge, y avisar en el grupo para
-- que todos vuelvan a correr "migrate ... up" desde cero en su entorno
-- local si hace falta.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE courses (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    teacher_id             UUID NOT NULL,
    slug                   TEXT NOT NULL UNIQUE,
    published_version_id   UUID,
    latest_draft_version_id UUID,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE course_versions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id      UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    version_number INT NOT NULL,
    status         TEXT NOT NULL CHECK (status IN ('draft', 'published', 'superseded')),
    title          TEXT NOT NULL DEFAULT '',
    summary        TEXT NOT NULL DEFAULT '',
    category       TEXT NOT NULL DEFAULT '',
    change_type    TEXT CHECK (change_type IN ('minor', 'major')),
    published_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (course_id, version_number)
);

-- Las columnas de courses que apuntan a course_versions se llenan después
-- de crear la primera versión (ver CreateCourseWithFirstVersion en el
-- código), por eso no llevan FK: crear la FK circular entre las dos
-- tablas en la misma migración complica el orden de creación sin
-- aportar mucho para el MVP. Si más adelante se quiere reforzar esto a
-- nivel de base de datos, se puede agregar con un ALTER TABLE separado.

CREATE TABLE modules (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    course_version_id UUID NOT NULL REFERENCES course_versions(id) ON DELETE CASCADE,
    stable_id         UUID NOT NULL DEFAULT gen_random_uuid(),
    title             TEXT NOT NULL,
    position          INT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (course_version_id, position)
);

CREATE TABLE units (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    module_id  UUID NOT NULL REFERENCES modules(id) ON DELETE CASCADE,
    stable_id  UUID NOT NULL DEFAULT gen_random_uuid(),
    title      TEXT NOT NULL,
    position   INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (module_id, position)
);

CREATE TABLE resources (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    unit_id                UUID NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    stable_id              UUID NOT NULL DEFAULT gen_random_uuid(),
    type                   TEXT NOT NULL CHECK (type IN (
                               'rich_text', 'image', 'video', 'audio', 'pdf',
                               'presentation', 'file', 'iframe', 'link', 'quiz'
                           )),
    title                  TEXT NOT NULL,
    position               INT NOT NULL,
    visible                BOOLEAN NOT NULL DEFAULT true,
    required               BOOLEAN NOT NULL DEFAULT true,
    downloadable           BOOLEAN NOT NULL DEFAULT false,
    processing_status      TEXT CHECK (processing_status IN ('pending', 'processing', 'ready', 'failed')),
    content_markdown       TEXT,
    content_draft_markdown TEXT,
    draft_saved_at         TIMESTAMPTZ,
    external_url           TEXT,
    media_asset_id         UUID,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (unit_id, position)
);

CREATE TABLE enrollments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id   UUID NOT NULL,
    course_id    UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    status       TEXT NOT NULL CHECK (status IN ('active', 'withdrawn')) DEFAULT 'active',
    enrolled_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    withdrawn_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (student_id, course_id)
);

CREATE INDEX idx_course_versions_course_id ON course_versions (course_id);
CREATE INDEX idx_modules_course_version_id ON modules (course_version_id);
CREATE INDEX idx_units_module_id ON units (module_id);
CREATE INDEX idx_resources_unit_id ON resources (unit_id);
CREATE INDEX idx_enrollments_course_id ON enrollments (course_id);
CREATE INDEX idx_enrollments_student_id ON enrollments (student_id);

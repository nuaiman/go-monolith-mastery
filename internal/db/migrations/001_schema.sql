-- ============================================================
-- USERS
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id               UUID PRIMARY KEY DEFAULT uuidv7(),

    role             VARCHAR(16) NOT NULL DEFAULT 'user'
                     CHECK (role IN ('user', 'admin', 'superadmin')),

    is_active        BOOLEAN NOT NULL DEFAULT TRUE,

    email            VARCHAR(128) UNIQUE,
    password         TEXT,
    refresh_token    TEXT,
    refresh_token_at TIMESTAMPTZ,
    name             VARCHAR(128),
    image_url        VARCHAR(2048),

    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- POSTS
-- ============================================================
CREATE TABLE IF NOT EXISTS posts (
    id         UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    title      VARCHAR(256) NOT NULL,
    body       TEXT NOT NULL,
    image_url  VARCHAR(2048),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- INDEXES
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_users_refresh_token
    ON users(refresh_token) WHERE refresh_token IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_posts_user_id ON posts(user_id);
CREATE INDEX IF NOT EXISTS idx_posts_created_at ON posts(created_at DESC);
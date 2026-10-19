-- Hesaplar ve panel girişi
CREATE TABLE users (
    id                INTEGER PRIMARY KEY,
    email             TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    password_hash     TEXT    NOT NULL,
    email_verified_at INTEGER,
    locale            TEXT    NOT NULL DEFAULT 'tr',
    created_at        INTEGER NOT NULL
) STRICT;

CREATE TABLE auth_sessions (
    token_hash BLOB    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL
) STRICT;

CREATE TABLE email_tokens (
    token_hash BLOB    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose    TEXT    NOT NULL CHECK (purpose IN ('verify', 'reset', 'email_change')),
    new_email  TEXT,
    expires_at INTEGER NOT NULL
) STRICT;

-- Siteler ve takım
CREATE TABLE sites (
    id         TEXT    PRIMARY KEY,
    domain     TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    public     INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
) STRICT;

CREATE TABLE memberships (
    site_id TEXT    NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role    TEXT    NOT NULL CHECK (role IN ('owner', 'admin', 'viewer')),
    PRIMARY KEY (site_id, user_id)
) STRICT;

CREATE TABLE invites (
    token_hash BLOB    PRIMARY KEY,
    site_id    TEXT    NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    email      TEXT    NOT NULL COLLATE NOCASE,
    role       TEXT    NOT NULL CHECK (role IN ('admin', 'viewer')),
    invited_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    expires_at INTEGER NOT NULL
) STRICT;

-- Hedefler ve huniler
CREATE TABLE goals (
    id      TEXT PRIMARY KEY,
    site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    kind    TEXT NOT NULL CHECK (kind IN ('page', 'event')),
    value   TEXT NOT NULL,
    UNIQUE (site_id, kind, value)
) STRICT;

CREATE TABLE funnels (
    id      TEXT PRIMARY KEY,
    site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    name    TEXT NOT NULL
) STRICT;

CREATE TABLE funnel_steps (
    funnel_id TEXT    NOT NULL REFERENCES funnels(id) ON DELETE CASCADE,
    position  INTEGER NOT NULL,
    kind      TEXT    NOT NULL CHECK (kind IN ('page', 'event')),
    value     TEXT    NOT NULL,
    PRIMARY KEY (funnel_id, position)
) STRICT;

-- Günlük tuz: sadece bugünkü satır durur
CREATE TABLE salts (
    day   TEXT PRIMARY KEY,
    value BLOB NOT NULL
) STRICT;

-- Ziyaretler ve olaylar
CREATE TABLE sessions (
    id           INTEGER PRIMARY KEY,
    site_id      TEXT    NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    visitor      INTEGER NOT NULL,
    started_at   INTEGER NOT NULL,
    ended_at     INTEGER NOT NULL,
    pageviews    INTEGER NOT NULL DEFAULT 0,
    entry_page   TEXT    NOT NULL DEFAULT '',
    exit_page    TEXT    NOT NULL DEFAULT '',
    referrer     TEXT    NOT NULL DEFAULT '',
    utm_source   TEXT    NOT NULL DEFAULT '',
    utm_medium   TEXT    NOT NULL DEFAULT '',
    utm_campaign TEXT    NOT NULL DEFAULT '',
    country      TEXT    NOT NULL DEFAULT '',
    device       TEXT    NOT NULL DEFAULT '',
    browser      TEXT    NOT NULL DEFAULT '',
    os           TEXT    NOT NULL DEFAULT ''
) STRICT;
CREATE INDEX sessions_site_time ON sessions (site_id, started_at);

CREATE TABLE events (
    id         INTEGER PRIMARY KEY,
    site_id    TEXT    NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    ts         INTEGER NOT NULL,
    session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    pathname   TEXT    NOT NULL
) STRICT;
CREATE INDEX events_site_time ON events (site_id, ts);
CREATE INDEX events_session ON events (session_id);

CREATE TABLE event_props (
    event_id INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    site_id  TEXT    NOT NULL,
    key      TEXT    NOT NULL,
    value    TEXT    NOT NULL
) STRICT;
CREATE INDEX event_props_site_key ON event_props (site_id, key, value);
CREATE INDEX event_props_event ON event_props (event_id);

-- Ödeme (cloud modu)
CREATE TABLE subscriptions (
    user_id            INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    plan               TEXT    NOT NULL,
    status             TEXT    NOT NULL,
    iyzico_ref         TEXT,
    current_period_end INTEGER
) STRICT;

CREATE TABLE webhook_log (
    event_id    TEXT    PRIMARY KEY,
    received_at INTEGER NOT NULL
) STRICT;

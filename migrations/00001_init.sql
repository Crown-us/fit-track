-- +goose Up
CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    created_at    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE profiles (
    user_id             INTEGER PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    sex                 TEXT    NOT NULL CHECK (sex IN ('male', 'female')),
    birth_date          TEXT    NOT NULL,
    height_cm           REAL    NOT NULL CHECK (height_cm > 0),
    weight_kg           REAL    NOT NULL CHECK (weight_kg > 0),
    activity_level      TEXT    NOT NULL,
    goal                TEXT    NOT NULL CHECK (goal IN ('lose', 'maintain', 'gain')),
    weekly_workout_goal INTEGER NOT NULL CHECK (weekly_workout_goal >= 0),
    updated_at          TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- owner_id NULL marks a seeded, global, read-only food.
CREATE TABLE foods (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id            INTEGER REFERENCES users (id) ON DELETE CASCADE,
    name                TEXT    NOT NULL,
    kcal_per_100g       REAL    NOT NULL CHECK (kcal_per_100g >= 0),
    protein_g_per_100g  REAL    NOT NULL CHECK (protein_g_per_100g >= 0),
    carbs_g_per_100g    REAL    NOT NULL CHECK (carbs_g_per_100g >= 0),
    fat_g_per_100g      REAL    NOT NULL CHECK (fat_g_per_100g >= 0),
    source              TEXT    NOT NULL,
    verification_status TEXT    NOT NULL
        CHECK (verification_status IN ('verified', 'needs_verification', 'user_provided')),
    created_at          TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Makes seeding idempotent: a global food name can exist only once.
CREATE UNIQUE INDEX idx_foods_global_name ON foods (name) WHERE owner_id IS NULL;
CREATE INDEX idx_foods_owner ON foods (owner_id);

CREATE TABLE workouts (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    activity        TEXT    NOT NULL,
    duration_min    REAL    NOT NULL CHECK (duration_min > 0),
    distance_km     REAL    CHECK (distance_km IS NULL OR distance_km >= 0),
    performed_on    TEXT    NOT NULL,
    calories_burned REAL    NOT NULL CHECK (calories_burned >= 0),
    created_at      TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX idx_workouts_user_date ON workouts (user_id, performed_on);

CREATE TABLE meals (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    eaten_on   TEXT    NOT NULL,
    meal_type  TEXT    NOT NULL CHECK (meal_type IN ('breakfast', 'lunch', 'dinner', 'snack')),
    created_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX idx_meals_user_date ON meals (user_id, eaten_on);

-- food_id has no cascade on purpose: a food that is part of a logged meal
-- must not disappear and silently change past totals.
CREATE TABLE meal_items (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    meal_id INTEGER NOT NULL REFERENCES meals (id) ON DELETE CASCADE,
    food_id INTEGER NOT NULL REFERENCES foods (id),
    grams   REAL    NOT NULL CHECK (grams > 0)
);

CREATE INDEX idx_meal_items_meal ON meal_items (meal_id);
CREATE INDEX idx_meal_items_food ON meal_items (food_id);

-- +goose Down
DROP TABLE meal_items;
DROP TABLE meals;
DROP TABLE workouts;
DROP TABLE foods;
DROP TABLE profiles;
DROP TABLE users;

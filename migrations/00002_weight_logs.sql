-- +goose Up
-- One weight per user per day; logging the same day again replaces it.
CREATE TABLE weight_logs (
    user_id   INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    logged_on TEXT    NOT NULL,
    weight_kg REAL    NOT NULL CHECK (weight_kg > 0),
    PRIMARY KEY (user_id, logged_on)
);

-- Existing profiles get their current weight as the first history entry, so
-- every profile has at least one weight log from here on.
INSERT INTO weight_logs (user_id, logged_on, weight_kg)
SELECT user_id, substr(updated_at, 1, 10), weight_kg FROM profiles;

-- +goose Down
DROP TABLE weight_logs;

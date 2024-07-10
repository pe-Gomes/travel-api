-- Write your migrate up statements here
CREATE TABLE IF NOT EXISTS activities (
    "id" uuid PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
    "trip_id" uuid NOT NULL,
    "title" varchar(255) NOT NULL,
    "occurs_at" timestamp NOT NULL,

    FOREIGN KEY (trip_id) REFERENCES trips (
        id
    ) ON UPDATE CASCADE ON DELETE CASCADE
);
---- create above / drop below ----
DROP TABLE IF EXISTS activities;
-- Write your migrate down statements here. If this migration is irreversible
-- Then delete the separator line above.

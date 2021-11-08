CREATE TABLE statuses (
  id    INTEGER PRIMARY KEY,
  label TEXT NOT NULL,
  color TEXT NOT NULL -- hex format such as #ff0044
);

CREATE TABLE events (
  id          INTEGER PRIMARY KEY,
  title       TEXT NOT NULL,
  occurs_at   DATETIME NOT NULL,
  description TEXT,
  status      INTEGER NOT NULL REFERENCES statuses (id)
);

CREATE TABLE guests (
  id    INTEGER PRIMARY KEY,
  name  TEXT NOT NULL,
  email TEXT NOT NULL UNIQUE
);

CREATE TABLE participations (
  guest_id INTEGER NOT NULL REFERENCES guests (id) ON DELETE CASCADE,
  event_id INTEGER NOT NULL REFERENCES events (id) ON DELETE CASCADE,
  attend   INTEGER NOT NULL,

  PRIMARY KEY (guest_id, event_id)
);

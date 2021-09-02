CREATE TABLE statuses (
  id    INTEGER PRIMARY KEY,
  label TEXT NOT NULL,
  color TEXT -- hex format such as #ff0044
);

CREATE TABLE events (
  id     INTEGER PRIMARY KEY,
  title  TEXT NOT NULL,
  desc   TEXT,
  status INTEGER NOT NULL REFERENCES statuses (id) ON DELETE SET NULL
);

CREATE TABLE guests (
  id    INTEGER PRIMARY KEY,
  name  TEXT NOT NULL,
  email TEXT NOT NULL
);

CREATE TABLE participations (
  guest_id INTEGER NOT NULL REFERENCES guests (id) ON DELETE CASCADE,
  event_id INTEGER NOT NULL REFERENCES events (id) ON DELETE CASCADE,
  assist   INTEGER NOT NULL, -- 0 no, 1 yes, 2 if needed

  PRIMARY KEY (guest_id, event_id)
);

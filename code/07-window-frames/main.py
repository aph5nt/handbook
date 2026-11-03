"""Window frames, LAG and LEAD. Run in SQLite.

Tab 1: a 7-day moving average, a frame per row, the default running total and the RANGE tie trap.
Tab 2: LAG and LEAD, the daily change, and sessions from gaps.
Tab 3: a missing day stretches a ROWS frame; a calendar join fixes it.
Run: python3 main.py
"""
import sqlite3

SETUP = """
CREATE TABLE daily_volume (day INT, volume INT);
INSERT INTO daily_volume VALUES (1, 10), (2, 12), (3, 8), (4, 14), (5, 9), (6, 11), (7, 13), (8, 15);
CREATE TABLE tied (day INT, volume INT);
INSERT INTO tied VALUES (1, 10), (2, 12), (3, 8), (3, 10), (4, 14);
CREATE TABLE transfers (ts TEXT);
INSERT INTO transfers VALUES ('10:00'), ('10:05'), ('10:20'), ('11:30'), ('11:34');
CREATE TABLE gappy (day INT, volume INT);
INSERT INTO gappy VALUES (1, 10), (2, 12), (3, 8), (5, 9), (6, 11), (7, 13), (8, 15), (9, 7);
"""

MOVING_AVG = """
-- each row gets its own frame: itself and the six rows before it
SELECT day, volume,
       ROUND(AVG(volume) OVER (ORDER BY day
             ROWS BETWEEN 6 PRECEDING AND CURRENT ROW), 1) AS avg7
FROM daily_volume ORDER BY day;
"""

RUNNING = """
-- no frame written: from the first row to this one
SELECT day, volume, SUM(volume) OVER (ORDER BY day) AS total
FROM daily_volume ORDER BY day;
"""

RANGE_TRAP = """
-- the default frame is RANGE: rows with the same day are peers
SELECT day, volume,
       SUM(volume) OVER (ORDER BY day) AS range_total,
       SUM(volume) OVER (ORDER BY day ROWS UNBOUNDED PRECEDING) AS rows_total
FROM tied ORDER BY day, volume;
"""

LAG_LEAD = """
-- LAG reaches one row back, LEAD one row forward
SELECT day, volume,
       LAG(volume) OVER (ORDER BY day) AS prev,
       volume - LAG(volume) OVER (ORDER BY day) AS change,
       LEAD(volume) OVER (ORDER BY day) AS next
FROM daily_volume ORDER BY day;
"""

SESSIONS = """
-- minutes since the last transfer, over 30 starts a new session
WITH g AS (
  SELECT ts, (strftime('%s', '2026-10-01 ' || ts)
            - strftime('%s', '2026-10-01 ' || LAG(ts) OVER (ORDER BY ts))) / 60 AS gap_min
  FROM transfers
)
SELECT ts, gap_min,
       CASE WHEN gap_min IS NULL OR gap_min > 30 THEN 'new' ELSE 'same' END AS session
FROM g ORDER BY ts;
"""

GAP_TRAP = """
-- day 4 has no row: six rows back from day 9 reach day 2
SELECT day, volume,
       ROUND(AVG(volume) OVER (ORDER BY day
             ROWS BETWEEN 6 PRECEDING AND CURRENT ROW), 1) AS avg7,
       MIN(day) OVER (ORDER BY day
             ROWS BETWEEN 6 PRECEDING AND CURRENT ROW) AS first_day
FROM gappy ORDER BY day;
"""

CALENDAR_FIX = """
-- fill the gaps first: one row per calendar day, then frame
WITH RECURSIVE calendar(day) AS (
  SELECT 1 UNION ALL SELECT day + 1 FROM calendar WHERE day < 9
),
filled AS (
  SELECT c.day, COALESCE(g.volume, 0) AS volume
  FROM calendar c LEFT JOIN gappy g ON g.day = c.day
)
SELECT day, volume,
       ROUND(AVG(volume) OVER (ORDER BY day
             ROWS BETWEEN 6 PRECEDING AND CURRENT ROW), 1) AS avg7,
       MIN(day) OVER (ORDER BY day
             ROWS BETWEEN 6 PRECEDING AND CURRENT ROW) AS first_day
FROM filled ORDER BY day;
"""


def show(db, label, sql):
    print(label)
    for row in db.execute(sql):
        print("  ", row)


def main():
    db = sqlite3.connect(":memory:")
    db.executescript(SETUP)
    show(db, "moving average (day, volume, avg7):", MOVING_AVG)
    show(db, "no frame (day, volume, total):", RUNNING)
    show(db, "tie on day 3 (day, volume, RANGE total, ROWS total):", RANGE_TRAP)
    print()
    show(db, "LAG and LEAD (day, volume, prev, change, next):", LAG_LEAD)
    show(db, "sessions (ts, gap in minutes, session):", SESSIONS)
    print()
    show(db, "missing day 4 (day, volume, avg7, first day in frame):", GAP_TRAP)
    show(db, "calendar fix (day, volume, avg7, first day in frame):", CALENDAR_FIX)


if __name__ == "__main__":
    main()

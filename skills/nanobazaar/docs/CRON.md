# Cron Polling

Cron exists to let operators run polling on a schedule when a persistent HEARTBEAT loop is not practical.

Important:
- Cron is NOT auto-installed.
- Scheduling is opt-in and only happens when you run `/nanobazaar cron enable`.

Command behavior (conceptual):
- `/nanobazaar cron enable` installs a cron entry that runs `/nanobazaar poll` on a schedule.
- `/nanobazaar cron disable` removes the previously installed cron entry.

Cron modes:
- Isolated session: cron launches a short-lived OpenClaw session that polls, processes, and exits.
- Main-session trigger: cron notifies a running session to perform a poll (no new session).

Recommended defaults:
- Run every 2-5 minutes to balance latency and cost.
- Use isolated session mode unless you already run a persistent OpenClaw session.

# Moving background jobs to River

River runs in-process with its own SQLite file (`river.db`, see `river/river.go`).
The UI is mounted at `/river`. Candidates below are in priority order.

## 1. Portfolio snapshot (weekdays 13:15 Vancouver)

- **Was:** `robfig/cron` entry in `handlers.go` calling `stock.SnapshotPortfolios()`.
- **Now:** River periodic job `portfolio_snapshot` (`workers/portfoliosnapshot`).
- **Why first:** sends no IRC messages, and `UpsertPortfolioSnapshot` is keyed on
  `(date, nick)`, so retries are safe.
- **Notes:** the date is fixed when the schedule fires and carried in the job
  args, so a retry still writes to the right day. The job is all or nothing:
  it fetches every price first (each symbol once, in parallel), fails and
  retries if any fetch fails, and only then writes every snapshot in one
  transaction. It allows 12 attempts (about 11 hours of retries), which ends
  before the next market open, so a retry still gets the closing price.
- **Status:** done.

## 2. "On this day" (daily 14:15:16 Vancouver)

- **Is:** `robfig/cron` entry in `handlers.go`. Picks a random note from this
  day in a past year, credits the author with `stock.Credit`, posts to the channel.
- **Plan:** River periodic job. Removing it drops the last use of `robfig/cron`.
- **Watch out:** not idempotent. A retry after a partial failure could pay or
  post twice. Make it record that it ran for the day (or split pay and post
  into separate steps) before allowing retries. The worker needs the `*bot.Bot`
  to post.

## 3. Reminders

- **Is:** `RemindMe` writes a row to `reminders`; `b.Repeat(10s, DoRemind)`
  polls for due rows and posts them.
- **Plan:** `RemindMe` inserts a River job with `ScheduledAt: at`; the worker posts.
- **Watch out:** `river.db` is a separate file, so the reminder row and its job
  can't share a transaction. Either keep both and accept they may disagree, or
  drop the table and use River's job list. Check whether anything lists or
  cancels reminders.

## 4. Low value, probably skip

- `handlers/nice.go`: 10s-delayed "nice" reply in a goroutine, lost on restart.
- Join handlers in `bot/irc.go` (`SendLaters`, `SendMissed`, `greetNewUser`):
  immediate reactions to a join; retries add little. Separately, `SendMissed`
  errors `panic` and take the bot down; worth fixing regardless.

## Not candidates

- Outgoing message rate limiter (`bot/irc.go`): in-memory, tied to the connection.
- Idle repeaters such as `POM` (`bot/idle`): depend on in-memory channel activity.
- Piss watcher: long-lived stream subscription, not a unit of work.
- Parallel stock quote fetches: concurrency within one request.
- `Catchup` sleeps: message pacing inside a reply.

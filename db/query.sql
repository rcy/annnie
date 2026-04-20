-- name: InsertVisit :exec
insert into visits(session, note_id) values(?,?);

-- name: Link :one
select * from notes where id = ? and kind = 'link';

-- name: InsertNickWeatherRequest :exec
insert into nick_weather_requests(nick, query, city, country) values(?,?,?,?);

-- name: LastNickWeatherRequest :one
select * from nick_weather_requests where nick = ? order by created_at desc limit 1;

-- name: LastWeatherRequestByPrefix :one
select * from nick_weather_requests where city like ? || '%' order by created_at desc limit 1;

-- name: InsertNote :one
insert into notes(target, nick, kind, text, anon) values(?,?,?,?,?) returning *;

-- name: LastDaysNotes :many
select created_at, nick, text, kind from notes where created_at > datetime('now', '-1 day') order by created_at asc;

-- name: UnsentAnonymousNotes :many
select * from notes where created_at <= ? and kind = ? and nick = target order by id asc limit 420;

-- name: MarkAnonymousNoteDelivered :one
update notes set target = ?, created_at = current_timestamp where id = ? returning *;

-- name: YoutubeLinks :many
select * from notes where kind = 'link' and text like '%youtube.com%' or text like '%youtu.be%';

-- name: AllNotes :many
select * from notes where target != nick order by created_at desc limit 10000;

-- name: AllNickNotes :many
select * from notes where target != nick and nick = ? order by created_at desc limit 10000;

-- name: NotesBetween :many
select *
from notes
where
  created_at >= @start_at and created_at < @end_at
order by created_at asc;

-- name: NicksWithNoteCount :many
select nick, count(nick) as count from notes group by nick;

-- name: ChannelNick :one
select * from channel_nicks where present = ? and channel = ? and nick = ? collate nocase;

-- name: ChannelNotesSince :many
select * from notes where target = ? and created_at > ? order by created_at asc limit 69;

-- name: CreateGeneratedImage :one
insert into generated_images(filename, prompt, revised_prompt) values(?,?,?) returning *;

-- name: RandomHistoricalTodayNote :one
select * from notes
where
  strftime('%m-%d', created_at) = strftime('%m-%d', 'now')
and
  strftime('%Y', created_at) != strftime('%Y', 'now')
order by random()
limit 1;

-- name: NoteByID :one
select * from notes where id = ?;

-- name: UpdateNoteTextByID :one
update notes set text = ? where id = ? returning *;

-- name: DeleteNoteByID :exec
delete from notes where id = ?;

-- name: NickBySession :one
select * from nick_sessions where session = ?;

-- name: CreateNickSession :exec
insert into nick_sessions(nick, session) values(?,?);

-- name: DeleteNickSessions :exec
delete from nick_sessions where nick = ?;

-- name: GeneratedImageByID :one
select * from generated_images where id = ?;

-- name: GeneratedImages :many
select * from generated_images order by created_at desc;

-- name: Notes :many
select * from notes where kind='note' order by created_at desc;

-- name: NonAnonNotes :many
select * from notes where kind='note' and nick != target order by created_at desc;

-- name: NotesAndQuotes :many
select * from notes where kind='note' or kind='quote' order by created_at desc;

-- name: ScheduleFutureMessage :one
insert into future_messages(kind) values(?) returning *;

-- name: ReadyFutureMessage :one
select * from future_messages where datetime('now') > datetime(created_at, ?) limit 1;

-- name: DeleteFutureMessage :exec
delete from future_messages where id = ?;

-- name: CacheLoad :one
select * from cache where key = @key;

-- name: CacheStore :one
insert into cache(key, value) values(@key, @value) returning *;

-- name: CacheRemove :exec
delete from cache where key = @key;

-- name: InsertFile :one
insert into files(nick,content) values (@nick, @content) returning *;

-- name: GetFile :one
select * from files where id = @id;

-- name: ListFiles :many
select id, created_at, nick from files order by created_at desc;

-- name: UpdateNickTimezone :exec
update nick_timezones set tz = @tz where nick = @nick;

-- name: InsertNickTimezone :exec
insert into nick_timezones(tz, nick) values(@tz, @nick);

-- name: GetNickTimezone :one
select * from nick_timezones where nick = @nick;

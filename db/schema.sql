CREATE TABLE migration_version (
			version INTEGER
		);
CREATE TABLE links(created_at text, nick text, text text);
CREATE TABLE laters(created_at text, nick text, target text, message text, sent boolean default false);
CREATE TABLE notes(
  id INTEGER not null primary key,
  created_at datetime not null default current_timestamp,
  nick text,
  text text,
  kind text not null default 'note', target text not null default '', anon bool not null default false);
CREATE TABLE reminders(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  nick text not null,
  remind_at datetime not null,
  what text not null
);
CREATE TABLE revs(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  sha text not null
);
CREATE TABLE visits(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  session text not null,
  note_id integer references notes not null
);
CREATE TABLE nick_weather_requests(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  nick text not null,
  query text not null,
  city text not null,
  country text not null
);
CREATE TABLE IF NOT EXISTS "channel_nicks"(
  channel text not null,
  nick text not null,
  present bool not null default false,
  updated_at datetime not null
);
CREATE UNIQUE INDEX channel_nick_unique_index on "channel_nicks"(channel, nick);
CREATE TABLE generated_images(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  filename text not null,
  prompt text not null,
  revised_prompt text not null
);
CREATE TABLE nick_sessions(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  nick text not null,
  session text not null
);
CREATE TABLE bedtimes(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  nick text not null,
  message text
);
CREATE TABLE future_messages(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  kind text not null
);
CREATE TABLE cache(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  key text not null,
  value text not null
);
CREATE TABLE files(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  nick text not null,
  content blob not null,
  thumbnail blob
);
CREATE TABLE nick_timezones(
  nick text not null primary key,
  tz text not null
);
CREATE TABLE configs(
  key text not null primary key,
  value text not null,
  nick text not null
);

package db

import (
	"log"

	"github.com/BurntSushi/migration"
	"github.com/jmoiron/sqlx"

	_ "modernc.org/sqlite"
)

func Open(dbfile string) *sqlx.DB {
	log.Printf("Opening db: %s", dbfile)

	migrations := []migration.Migrator{
		func(tx migration.LimitedTx) error {
			_, err := tx.Exec(`create table if not exists notes(created_at text, nick text, text text)`)
			return err
		},
		func(tx migration.LimitedTx) error {
			_, err := tx.Exec(`create table if not exists links(created_at text, nick text, text text)`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: adding kind column to notes")
			_, err := tx.Exec(`alter table notes add column kind string not null default "note"`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: adding laters table")
			_, err := tx.Exec(`create table laters(created_at text, nick text, target text, message text, sent boolean default false)`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: adding channel_nicks table")
			_, err := tx.Exec(`create table channel_nicks(channel text not null, nick text not null, present bool not null default false)`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add unique constrant to channel_nicks table")

			// delete duplicates, keeping oldest records
			_, err := tx.Exec(`delete from channel_nicks where rowid not in (select min(rowid) from channel_nicks group by nick, channel)`)
			if err != nil {
				return err
			}

			// add unique constraint
			_, err = tx.Exec(`create unique index channel_nick_unique_index on channel_nicks(channel, nick)`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add primary key to notes")
			_, err := tx.Exec(`
pragma foreign_key = off;

alter table notes rename to old_notes;

create table notes(
  id INTEGER not null primary key,
  created_at datetime not null default current_timestamp,
  nick text,
  text text,
  kind string not null default "note"
);

insert into notes select rowid, * from old_notes;

drop table old_notes;

pragma foreign_key = on;
`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add seen table")
			_, err := tx.Exec(`
create table seen_by(
  created_at datetime not null default current_timestamp,
  note_id references notes not null,
  nick text not null
);`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add updated_at to channel_nicks")
			_, err := tx.Exec(`alter table channel_nicks add column updated_at text`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: transactions table")
			_, err := tx.Exec(`
create table transactions(
  created_at datetime not null default current_timestamp,
  nick text not null,
  verb text not null,
  symbol text not null,
  shares number not null,
  price number not null
);`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: reminders table")
			_, err := tx.Exec(`
create table reminders(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  nick text not null,
  remind_at datetime not null,
  what text not null
);`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: revs table")
			_, err := tx.Exec(`
create table revs(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  sha text not null
);`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add target to notes")
			_, err := tx.Exec(`
alter table notes add column target string not null default "";
`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: drop transactions")
			_, err := tx.Exec(`drop table transactions;`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: drop seen_by")
			_, err := tx.Exec(`drop table seen_by;`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: fix notes.kind")
			_, err := tx.Exec(`
alter table notes add column kindx text not null default 'note';
update notes set kindx = kind;
alter table notes drop column kind;
alter table notes rename column kindx to kind;
`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: fix notes.kind")
			_, err := tx.Exec(`
alter table notes add column targetx text not null default '';
update notes set targetx = target;
alter table notes drop column target;
alter table notes rename column targetx to target;
`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: create visits")
			_, err := tx.Exec(`
create table visits(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  session text not null,
  note_id integer references notes not null
);
`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: create nick_weather_requests")
			_, err := tx.Exec(`
create table nick_weather_requests(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  nick text not null,
  query text not null,
  city text not null,
  country text not null
);
`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: fix channel_nicks.updated_at")
			_, err := tx.Exec(`
drop index channel_nick_unique_index;

create table new_channel_nicks(
  channel text not null,
  nick text not null,
  present bool not null default false,
  updated_at datetime not null
);
create unique index channel_nick_unique_index on new_channel_nicks(channel, nick);

insert into new_channel_nicks select * from channel_nicks;
drop table channel_nicks;
alter table new_channel_nicks rename to channel_nicks;
`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add anon to notes")
			_, err := tx.Exec(`
alter table notes add column anon bool not null default false;
`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add generated_images table")
			_, err := tx.Exec(`
create table generated_images(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  filename text not null,
  prompt text not null,
  revised_prompt text not null
);
`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add nick_sessions table")
			_, err := tx.Exec(`
create table nick_sessions(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  nick text not null,
  session text not null
);
`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add bedtimes table")
			_, err := tx.Exec(`
create table bedtimes(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  nick text not null,
  message text
);
`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add future messages")
			_, err := tx.Exec(`
create table future_messages(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  kind text not null
);
`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add cache")
			_, err := tx.Exec(`
create table cache(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  key text not null,
  value text not null
);
`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add files")
			_, err := tx.Exec(`
create table files(
  id integer not null primary key,
  created_at datetime not null default current_timestamp,
  nick text not null,
  content blob not null
);
`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add nick_timezones")
			_, err := tx.Exec(`
create table nick_timezones(
  nick text not null primary key,
  tz text not null
);
`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add thumbnail to files")
			_, err := tx.Exec(`alter table files add column thumbnail blob`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add configs")
			_, err := tx.Exec(`
create table configs(
  key text not null primary key,
  value text not null,
  nick text not null
);
`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add mime to files")
			_, err := tx.Exec(`alter table files add column mime text`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add covering index for files listing")
			_, err := tx.Exec(`create index idx_files_listing on files(created_at desc, id, nick, mime)`)
			return err
		},
		func(tx migration.LimitedTx) error {
			log.Println("MIGRATE: add og columns to notes")
			_, err := tx.Exec(`
alter table notes add column og_title text;
alter table notes add column og_description text;
alter table notes add column og_image text;
`)
			return err
		},
	}

	db, err := migration.Open("sqlite", dbfile, migrations)
	if err != nil {
		log.Fatalf("MIGRATION: %v", err)
	}

	for _, pragma := range []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			log.Fatalf("pragma: %v", err)
		}
	}

	return sqlx.NewDb(db, "sqlite")
}

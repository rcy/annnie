export BUILDKIT_PROGRESS=plain
export CGO_ENABLED=0

watch:
	. ./.env && . ./.env.local && air

build:
	go build -o tmp/main .

fmt:
	go fmt main.go

lint:
	golangci-lint run

sql:
	. ./.env && sqlite3 $$SQLITE_DB

test:
	set -a && . ./.env.test && go test ./...

dbuild:
	docker build -t annie .

run:
	docker run --env-file=.env -e SQLITE_DB=:memory: annie

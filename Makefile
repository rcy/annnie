export BUILDKIT_PROGRESS=plain
export CGO_ENABLED=0

watch: .env.local
	. ./.env && . ./.env.local && air

.env.local:
	>$@

build:
	go build -o tmp/main .

fmt:
	go fmt main.go

lint:
	docker run --rm -t -v $$(pwd):/app -w /app \
		--user $$(id -u):$$(id -g) \
		-v $$(go env GOCACHE):/.cache/go-build -e GOCACHE=/.cache/go-build \
		-v $$(go env GOMODCACHE):/.cache/mod -e GOMODCACHE=/.cache/mod \
		-v ~/.cache/golangci-lint:/.cache/golangci-lint -e GOLANGCI_LINT_CACHE=/.cache/golangci-lint \
		golangci/golangci-lint:v2.15.0 golangci-lint run

sql:
	. ./.env && sqlite3 $$SQLITE_DB

test:
	set -a && . ./.env.test && go test ./...

dbuild:
	docker build -t annie .

run:
	docker run --env-file=.env -e SQLITE_DB=:memory: annie

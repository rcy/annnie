FROM golang:1.26-bookworm AS builder
ARG rev=dev
WORKDIR /work
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags "-X goirc/commit.Rev=$rev" -o app .

FROM debian:bookworm
RUN apt-get update && apt-get install -y bsdgames ca-certificates curl pup jq ddate
WORKDIR /work
COPY --from=builder /work/app .
EXPOSE 8080
CMD ["./app"]

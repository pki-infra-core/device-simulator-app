APP := device-simulator
ENV_FILE ?= .env

ifneq (,$(wildcard $(ENV_FILE)))
include $(ENV_FILE)
export
endif

.PHONY: help fmt test build run bootstrap status ping telemetry reset renew

help:
	@printf "Targets:\n"
	@printf "  env         Copy examples/.env.example to .env if missing\n"
	@printf "  fmt         Run gofmt on all Go files\n"
	@printf "  test        Run go test ./...\n"
	@printf "  build       Build the CLI binary\n"
	@printf "  run         Run the CLI, pass ARGS='<command>'\n"
	@printf "  bootstrap   Run bootstrap command\n"
	@printf "  status      Run status command\n"
	@printf "  ping        Run ping command\n"
	@printf "  telemetry   Run telemetry command\n"
	@printf "  renew       Run renew command\n"
	@printf "  reset       Run reset command\n"
	@printf "\n"
	@printf "Environment:\n"
	@printf "  Put variables in .env or pass ENV_FILE=/path/to/file\n"

env:
	@if [ -f .env ]; then \
		printf ".env already exists\n"; \
	else \
		cp examples/.env.example .env; \
		printf "created .env from examples/.env.example\n"; \
	fi

fmt:
	@gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

test:
	@go test ./...

build:
	@go build -o bin/$(APP) ./cmd/device-simulator

run:
	@go run ./cmd/device-simulator $(ARGS)

bootstrap:
	@go run ./cmd/device-simulator bootstrap

status:
	@go run ./cmd/device-simulator status

ping:
	@go run ./cmd/device-simulator ping

telemetry:
	@go run ./cmd/device-simulator telemetry

renew:
	@go run ./cmd/device-simulator renew

reset:
	@go run ./cmd/device-simulator reset

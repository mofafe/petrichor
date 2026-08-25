.PHONY: fmt fmt-check vet test ci server-ci web-ci web web-lan server dev tidy go-download npm-install deploy-staging staging-up staging-check

fmt:
	cd apps/server && go fmt ./...

fmt-check:
	test -z "$$(gofmt -l apps/server)"

vet:
	cd apps/server && go vet ./...

test:
	cd apps/server && go test ./...

ci:
	$(MAKE) server-ci
	$(MAKE) web-ci

server-ci:
	$(MAKE) -j3 fmt-check vet test

web-ci:
	cd apps/web && npm ci && npm run typecheck && npm run typecheck:tsc

web:
	cd apps/web && npm run dev

web-lan:
	cd apps/web && npm run dev:lan

server:
	cd apps/server && go run ./cmd/server

dev:
	$(MAKE) -j2 web server

deploy-staging:
	docker compose -f compose.yaml -f compose.staging.yaml up -d --build

staging-up: deploy-staging

staging-check:
	@if [ -f .env ]; then \
		set -a; . ./.env; set +a; \
	fi; \
	./scripts/check-staging.sh

tidy:
	cd apps/server && go mod tidy

go-download:
	cd apps/server && go mod download

npm-install:
	cd apps/web && npm install

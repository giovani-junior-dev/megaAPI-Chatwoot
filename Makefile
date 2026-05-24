.PHONY: test integration lint build run clean security-scan loadtest-smoke chaos-smoke

test:
	go test ./...

integration:
	go test -tags=integration ./...

lint:
	go vet ./...
	golangci-lint run

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bridge ./cmd/bridge

run: build
	./bridge serve

clean:
	rm -f bridge bridge.exe coverage.out

security-scan:
	./deploy/security/gosec.sh
	@echo "Run TARGET_URL=<url> ./deploy/security/nuclei.sh and ./deploy/security/zap.sh against a live instance."

loadtest-smoke:
	./deploy/loadtest/run-smoke.sh

chaos-smoke:
	./deploy/chaos/chaos.sh

APP=cliproxy-feishu-monitor

.PHONY: test build release shellcheck clean

test:
	go test ./...

build:
	go build -o dist/$(APP) .

release:
	bash deploy/build-release.sh

shellcheck:
	bash -n deploy/install.sh
	bash -n deploy/deploy-from-tar.sh
	bash -n deploy/build-release.sh
	bash -n deploy/release.sh
	bash -n deploy/run-once.sh
	bash -n deploy/service-status.sh
	bash -n deploy/service-logs.sh

clean:
	rm -f dist/$(APP)

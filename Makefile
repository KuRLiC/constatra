.PHONY: build build-all release-dry-run test check clean

build:
	./scripts/build.sh

build-all:
	./scripts/build.sh --all

release-dry-run:
	./scripts/release.sh --dry-run

test:
	./scripts/setup.sh
	bash -c 'source scripts/lib.sh && "$${CONSTATRA_GO_BIN}" test ./...'

check:
	./scripts/setup.sh
	bash -c 'source scripts/lib.sh && "$${CONSTATRA_GO_BIN}" vet ./...'
	bash -c 'source scripts/lib.sh && "$${CONSTATRA_GO_BIN}" test -race ./...'

clean:
	rm -rf build

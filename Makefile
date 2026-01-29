EXECS   := $(wildcard cmd/*)
TARGETS := ${EXECS:cmd/%=%}

TESTA   := ${shell go list ./... | grep -v /cmd/ }

BRANCH   := ${shell git branch --show-current}
REVCNT   := ${shell git rev-list --count $(BRANCH) --}
REVHASH  := ${shell git log -1 --format="%h"}

GITTAG   := ${shell git tag --points-at HEAD}
ifeq ($(strip $(GITTAG)),)
  RELEASE=untagged
  RELSFX=$(REVHASH)
else
  RELEASE=$(GITTAG)
  RELSFX=$(RELEASE)
endif


LDFLAGS  := -s -w -X main.version=${BRANCH}.${REVCNT}.${REVHASH} -X main.release=${RELEASE}

all: check clean build

check: gen lint test race

cover:
	go test -coverprofile=cover.out ${TESTA} && \
	go tool cover -func=cover.out

gen:
	go generate ./...

lint:
	golangci-lint run ./...

test:
	go test -count 1 ${TESTA}

race:
	go test -race -count 1 ${TESTA} # need ginkgo cli for rerun

clean:
	rm -rf bin/*

build: ${TARGETS}
	@echo ":: Done"

${TARGETS}:
	@echo ":: Building $@"
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '${LDFLAGS}' -o bin/$@_linux-amd64_${RELSFX} cmd/$@/main.go
	@#CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags '${LDFLAGS}' -o bin/$@_linux-arm64_${RELSFX} cmd/$@/main.go
	@#CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags '${LDFLAGS}' -o bin/$@_darwin-arm64_${RELSFX} cmd/$@/main.go

image-%:
	@echo ":: Building local/$*:${RELSFX}"
	DOCKER_BUILDKIT=1 docker build --ssh default --platform linux/amd64 \
		--build-arg CMD=$(notdir $*) \
		--build-arg VERSION=${BRANCH}.${REVCNT}.${REVHASH} \
		--build-arg RELEASE=${RELEASE} \
		-t local/$*:${RELSFX} .

clean-image-%:
	@echo ":: Removing local/$*:${RELSFX}"
	-docker rmi local/$*:${RELSFX}

run-%:
	@echo ":: Running local/$*:${RELSFX} on port 3031"
	docker run --rm --network host --env-file secret.env -v $(PWD)/secret:/secret --name $(notdir $*) local/$*:${RELSFX}

.PHONY: all check cover gen lint test race clean build

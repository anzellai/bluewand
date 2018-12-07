BINARY = bluewand
VET_REPORT = vet.report
GOARCH = amd64

VERSION=0.0.1
COMMIT=$(shell git rev-parse HEAD)
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)

# Symlink into GOPATH
GITHUB_USERNAME=anzellai
BUILD_DIR=${GOPATH}/src/github.com/${GITHUB_USERNAME}/${BINARY}
CURRENT_DIR=$(shell pwd)
BUILD_DIR_LINK=$(shell readlink ${BUILD_DIR})
# Setup the -ldflags option for go build here, interpolate the variable values
LDFLAGS = -ldflags "-s -w -X main.VERSION=${VERSION} -X main.COMMIT=${COMMIT} -X main.BRANCH=${BRANCH}"

# Build the project
all: clean vet proto linux darwin

linux:
	cd ${BUILD_DIR}/server; \
	GOOS=linux GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BUILD_DIR}/bin/linux/${BINARY}-server . ; \
	cd ${BUILD_DIR}/client; \
	GOOS=linux GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BUILD_DIR}/bin/linux/${BINARY}-client . ; \
	cd - >/dev/null

darwin:
	cd ${BUILD_DIR}/server; \
	GOOS=darwin GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BUILD_DIR}/bin/darwin/${BINARY}-server . ; \
	cd ${BUILD_DIR}/client; \
	GOOS=darwin GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BUILD_DIR}/bin/darwin/${BINARY}-client . ; \
	cd - >/dev/null

rasberry3:
	cd ${BUILD_DIR}/server; \
	GOOS=linux GOARCH=arm GOARM=6 go build ${LDFLAGS} -o ${BUILD_DIR}/bin/rasberry3/${BINARY}-server . ; \
	cd ${BUILD_DIR}/client; \
	GOOS=linux GOARCH=arm GOARM=6 go build ${LDFLAGS} -o ${BUILD_DIR}/bin/rasberry3/${BINARY}-client . ; \
	cd - >/dev/null

vet:
	-cd ${BUILD_DIR}; \
	go vet ./... > ${VET_REPORT} 2>&1 ; \
	cd - >/dev/null

fmt:
	cd ${BUILD_DIR}; \
	go fmt $$(go list ./... | grep -v /vendor/) ; \
	cd - >/dev/null

proto:
	protoc -I bluewand/ bluewand/bluewand.proto --go_out=plugins=grpc:bluewand

build:
	make clean; \
	dep ensure -v; \
	go generate ${BUILD_DIR}/server/main.go; \
	make fmt; \
	make vet; \
	make linux; \
	make darwin;
	make rasberry3;

clean:
	-rm -f ${BUILD_DIR}/${VET_REPORT}
	-rm -rf ${BUILD_DIR}/bluewand/*.pb.go
	-rm -rf ${BUILD_DIR}/bin

.PHONY: linux darwin rasberry3 vet fmt clean proto build

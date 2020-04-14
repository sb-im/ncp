OS=
ARCH=
PROFIX=
VERSION=$(shell git describe --tags || echo "unknown version")
BUILDTIME=$(shell date)
GOBUILD=GOOS=$(OS) GOARCH=$(ARCH) \
				go build -ldflags '-X "sb.im/ncp/constant.Version=$(VERSION)" \
				-X "sb.im/ncp/constant.BuildTime=$(BUILDTIME)"'

all: build

build:
	$(GOBUILD)

run:
	go run `ls *.go | grep -v _test.go`

install:
	install -Dm755 ncp -t ${PROFIX}/usr/lib/ncp/
	install -Dm644 scripts/* -t ${PROFIX}/usr/lib/ncp/scripts/
	install -Dm644 conf/ncp.service -t ${PROFIX}/lib/systemd/system/
	install -Dm644 conf/ncp@.service -t ${PROFIX}/lib/systemd/system/
	install -Dm644 conf/config-dist.yml -t ${PROFIX}/etc/ncp/

test:
	go test -cover

clean:
	go clean


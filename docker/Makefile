#Makefile related to docker

GOHOSTARCH := $(shell go env GOHOSTARCH)
QEMU_VERSION ?= v2.12.0
QEMU_ARCH ?= amd64
GOARCH ?= $(shell go env GOARCH)
TARGET ?= library

DOCKER_IMAGE ?= gitea/gitea
DOCKER_TAG ?= latest
ifneq ($(TARGET), library)
  DOCKER_PLATFORM ?= linux-$(GOARCH)
else
  DOCKER_PLATFORM ?= $(shell docker version -f {{.Server.Os}}-{{.Server.Arch}} 2>/dev/null || echo 'undefined')
endif
DOCKER_REF ?= $(DOCKER_IMAGE):$(DOCKER_TAG)-$(DOCKER_PLATFORM)

.PHONY: docker
docker:
	docker build --disable-content-trust=false -t $(DOCKER_REF) .
# support also build args docker build --build-arg GITEA_VERSION=v1.2.3 --build-arg TAGS="bindata sqlite sqlite_unlock_notify"  .

.PHONY: docker-build
docker-build:
	docker run -ti --rm -v $(CURDIR):/srv/app/src/code.gitea.io/gitea -w /srv/app/src/code.gitea.io/gitea -e TAGS="bindata $(TAGS)" webhippie/golang:edge make clean generate build

.PHONY: docker-cross
docker-cross : docker-setup-qemu
	docker build --build-arg target=$(TARGET) --build-arg GOARCH=$(GOARCH) -f docker/Dockerfile.$(QEMU_ARCH) -t $(DOCKER_REF) .
#exemple TARGET=arm64v8 GOARCH=arm64 QEMU_ARCH=aarch64 make docker-cross
#exemple TARGET=amd64 GOARCH=amd64 QEMU_ARCH=amd64 make docker-cross
#exemple TARGET=arm32v6 GOARCH=arm QEMU_ARCH=arm make docker-cross

.PHONY: docker-setup-qemu
docker-setup-qemu:
	@if [ "$(QEMU_ARCH)" != ${GOHOSTARCH} ]; then \
	  echo "Loading qemu libs for multi-arch support."; \
	  (cd docker && curl -sL https://github.com/multiarch/qemu-user-static/releases/download/${QEMU_VERSION}/qemu-${QEMU_ARCH}-static.tar.gz | tar xz); \
	  docker run --rm --privileged multiarch/qemu-user-static:register --reset; \
		echo "Generate dockerfile for specific arch."; \
		sed "s/^#QEMU phase/COPY docker\/qemu-${QEMU_ARCH}-static* \/usr\/bin\//g" Dockerfile > docker/Dockerfile.${QEMU_ARCH}; \
		sed -i 's/^FROM $$target\/golang:1.10-alpine3.7 AS build-env/FROM karalabe\/xgo-latest AS build-env/g' docker/Dockerfile.${QEMU_ARCH}; \
		sed -i 's/^RUN apk --no-cache add build-base git/RUN GOARCH=$$GOHOSTARCH go get -u github.com\/jteeuwen\/go-bindata\/.../g' docker/Dockerfile.${QEMU_ARCH}; \
		sed -i 's/make clean generate build/PATH=$$PATH:\$$GOPATH\/bin\/ XGOARCH="${GOARCH}" make clean generate release-linux \&\& mv \/build\/gitea-* .\/gitea/g' docker/Dockerfile.${QEMU_ARCH}; \
	else cp Dockerfile docker/Dockerfile.${QEMU_ARCH}; \
	fi;

.PHONY: docker-setup-clean
docker-setup-clean:
	rm -f docker/qemu-*-static
	rm -f docker/Dockerfile.*

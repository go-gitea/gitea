#Makefile related to docker

DOCKER_IMAGE ?= gitea/gitea
DOCKER_TAG ?= latest
DOCKER_REF := $(DOCKER_IMAGE):$(DOCKER_TAG)


.PHONY: docker
docker:
	docker run -ti --rm -v $(CURDIR):/srv/app/src/code.gitea.io/gitea -w /srv/app/src/code.gitea.io/gitea -e TAGS="bindata $(TAGS)" webhippie/golang:edge make clean generate build
	docker build -t $(DOCKER_REF) .

.PHONY: docker-multi-arch-push-manifest
docker-multi-arch-push-manifest: DOCKER_MANIFEST ?= docker/manifest/base.yml
docker-multi-arch-push-manifest:
	@hash manifest-tool > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go get -u github.com/estesp/manifest-tool; \
	fi
	$(SED_INPLACE) "s;gitea/gitea;$(DOCKER_IMAGE);g" $(DOCKER_MANIFEST)
	@manifest-tool push from-spec $(DOCKER_MANIFEST)
	$(SED_INPLACE) "s;$(DOCKER_IMAGE);gitea/gitea;g" $(DOCKER_MANIFEST)

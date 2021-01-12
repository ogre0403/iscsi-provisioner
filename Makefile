# Copyright 2016 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

COMMIT_HASH = $(shell git rev-parse --short HEAD)
BRANCH_NAME = $(shell git rev-parse --abbrev-ref HEAD)
RET = $(shell git describe --contains $(COMMIT_HASH) 1>&2 2> /dev/null; echo $$?)
USER = $(shell whoami)
IS_DIRTY_CONDITION = $(shell git diff-index --name-status HEAD | wc -l)

ifeq ($(strip $(IS_DIRTY_CONDITION)), 0)
	# if clean,  IS_DIRTY tag is not required
	IS_DIRTY = $(shell echo "")
else
	# add dirty tag if repo is modified
	IS_DIRTY = $(shell echo "-dirty")
endif

# Image Tag rule
# 1. if repo in non-master branch, use branch name as image tag
# 2. if repo in a master branch, but there is no tag, use <username>-<commit-hash>
# 2. if repo in a master branch, and there is tag, use tag
ifeq ($(BRANCH_NAME), master)
	ifeq ($(RET),0)
		TAG = $(shell git describe --contains $(COMMIT_HASH))$(IS_DIRTY)
	else
		TAG = $(USER)-$(COMMIT_HASH)$(IS_DIRTY)
	endif
else
	TAG = $(BRANCH_NAME)$(IS_DIRTY)
endif


ifeq ($(REGISTRY),)
	REGISTRY = ogre0403/
endif

PROJ_NAME = iscsi-provisioner
IMAGE = $(REGISTRY)$(PROJ_NAME):$(TAG)

build:
	rm -rf bin/${PROJ_NAME}
	go mod vendor
	go build -mod=vendor  \
	-o bin/${PROJ_NAME}

run:
	bin/iscsi-provisioner start \
	--kubeconfig=/Users/ogre0403/.kube/config \
	--provisioner-name=iscsi-targetd \
	--targetd-address=140.110.30.57 \
	--log-level=debug

build-in-docker:
	rm -rf bin/${PROJ_NAME}
	CGO_ENABLED=0 GOOS=linux \
	go build -mod=vendor \
	-a -ldflags '-extldflags "-static"' \
	-o bin/${PROJ_NAME}

build-img:
	docker build -t $(IMAGE) -f ./Dockerfile .


clean:
	rm -rf bin/


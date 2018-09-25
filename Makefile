#
#  Copyright 2018 Nalej
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.
#

# Name of the target applications to be built
APPS=deployment-manager

# Target directory to store binaries and results
TARGET=bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test

# Docker configuration
DOCKER_REPO=nalej
VERSION=$(shell cat .version)



# Use ldflags to pass commit and branch information
# TODO: Integrate this into the compilation process
# Build information
COMMIT=$(shell git rev-parse HEAD)
#BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
# LDFLAGS to setup the version and commit. Notice that because of changes in go 1.10, we target a
# variable inside the main package that is being built.
LDFLAGS=-ldflags "-X main.MainVersion=${VERSION} -X main.MainCommit=${COMMIT}"

COVERAGE_FILE=$(TARGET)/coverage.out

.PHONY: all
all: dep build test image

.PHONY: dep
dep:
	if [ ! -d vendor ]; then \
	    echo ">>> Create vendor folder" ; \
	    mkdir vendor ; \
	fi ;
	$(info >>> Updating dependencies...)
	dep ensure -v

test-all: test test-race test-coverage

.PHONY: test test-race test-coverage
test:
	$(info >>> Launching tests...)
	$(GOTEST) ./...

test-race:
	$(info >>> Launching tests... (Race detector enabled))
	$(GOTEST) -race ./...

test-coverage:
	$(info >>> Launching tests... (Coverage enabled))
	$(GOTEST) -coverprofile=$(COVERAGE_FILE) -covermode=atomic  ./...

# Check the codestyle using gometalinter
.PHONY: checkstyle
checkstyle:
	gometalinter --disable-all --enable=golint --enable=vet --enable=errcheck --enable=goconst --vendor ./...

# Run go formatter
.PHONY: format
format:
	$(info >>> Formatting...)
	gofmt -s -w .

.PHONY: clean
clean:
	$(info >>> Cleaning project...)
	$(GOCLEAN)
	rm -Rf $(TARGET)

.PHONY: dep build-all build build-linux build-local
build-all: dep format build build-linux
build: dep local
build-linux: dep linux

# Local compilation
local:
	$(info >>> Building ...)
	for app in $(APPS); do \
            $(GOBUILD) $(LDFLAGS) -o $(TARGET)/"$$app" ./cmd/"$$app" ; \
	done

# Cross compilation to obtain a linux binary
linux:
	$(info >>> Bulding for Linux...)
	for app in $(APPS); do \
    	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(TARGET)/linux_amd64/"$$app" ./cmd/"$$app" ; \
	done

# Package all images and components
.PHONY: image image-create-dir create-image
image: build-linux image-create-dir create-image

image-create-dir:
	mkdir -p $(TARGET)/images

create-image:
	$(info >>> Creating images ...)
	for app in $(APPS); do \
        echo Create image of app $$app ; \
        if [ -f components/"$$app"/Dockerfile ]; then \
            mkdir -p $(TARGET)/images/"$$app" ; \
            docker build --no-cache -t $(DOCKER_REPO)/"$$app":$(VERSION) -f components/"$$app"/Dockerfile $(TARGET)/linux_amd64 ; \
            docker save $(DOCKER_REPO)/"$$app" > $(TARGET)/images/"$$app"/image.tar ; \
            // docker rmi $(DOCKER_REPO)/"$$app":$(VERSION) ; \
            cd $(TARGET)/images/"$$app"/ && tar cvzf "$$app".tar.gz * && cd - ; \
        else  \
            echo $$app has no Dockerfile ; \
        fi ; \
    done

# Publish the image
publish: image publish-image

publish-image:
	$(info >>> Publish images into Docker Hub ...)
	if [ ""$$DOCKER_USER"" = "" ]; then \
	    echo DOCKER_USER environment variable was not set!!! ; \
	    exit 1 ; \
	fi ; \
	if [ ""$$DOCKER_USER"" = "" ]; then \
        echo DOCKER_USER environment variable was not set!!! ; \
        exit 1 ; \
    fi ; \
	$(info >>> Assuming credentials are available in environment variables ...)
	echo  "$$DOCKER_PASSWORD" | docker login -u "$$DOCKER_USER" --password-stdin
	for app in $(APPS); do \
	    if [ -f $(TARGET)/images/"$$app"/image.tar ]; then \
	        docker push $(DOCKER_REPO)/"$$app":$(VERSION) ; \
	    else \
	        echo $$app has no image to be pushed ; \
	    fi ; \
   	    echo  Publish image of app $$app ; \
    done ; \
    docker logout ; \



VERSION?="0.1.0"
TEST?=./...
TESTARGS ?= -v
CURRDIR =$(shell pwd)
GODIR = $(CURRDIR)/server/src/github.com/xiyanxiyan10/quantcore
#CHECK_PACKAGE?=$$(go list ./... | grep -v migrations)
#GOFMT_FILES?=$$(find . -not -path "./pkg/model/migrations/*" -type f -name '*.go')
#WEBSITE_REPO=github.com/hashicorp/terraform-website
GOFMT_FILES =$$(find $(GODIR) -not -path "$(GODIR)/gobacktest" -type f -name '*.go')

default: test

clean:
	find ./ -name '*.out' |xargs rm -rf
	find ./ -name 'cscope.*' |xargs rm -rf
	find ./ -name 'tags'  |xargs rm -rf
	find ./ -name '*.so'  |xargs rm -rf
	find -name '*.o'   |xargs rm -rf
	find -name '*.a'   |xargs rm -rf
	find -name '*.d'   |xargs rm -rf
	find -name '*.pyc' |xargs rm -rf
	rm -rf golint-report.xml  coverage.out report.json govet-report.out

check: fmtcheck
	golangci-lint run  --config=./golangci.yml

#sonar:
#	golangci-lint run  --config=./golangci.yml --out-format checkstyle> golint-report.xml; \
#	export GSD_CONFIG_FILE=$(CURDIR)/configs/config.yml; \
#    	go test $(TEST) -coverprofile=coverage.out; \
#    	go test $(TEST) -json > report.json; \
#    	go list $(CHECK_PACKAGE) | xargs  go vet 2> govet-report.out

#check: fmtcheck vet lint staticcheck cyclo

bin: dev
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./build/apigateway -v ./cmd/example/...


#test: fmtcheck
#	export GSD_CONFIG_FILE=$(CURDIR)/configs/config.yml; \
#	go list  $(TEST) | xargs -t -n4  go test $(TESTARGS)   -timeout=2m -parallel=4


#testrace: fmtcheck
#	export GSD_CONFIG_FILE=$(CURDIR)/configs/config.yml; \
#	TF_ACC= go test -race $(TEST) $(TESTARGS)

#cover:
#	export GSD_CONFIG_FILE=$(CURDIR)/configs/config.yml; \
#	go test $(TEST) -coverprofile=coverage.out
#	go tool cover -html=coverage.out
#	rm coverage.out

vet: 
	go vet $(GOFMT_FILES) 

staticcheck:
	staticcheck $(GOFMT_FILES)

cyclo:
	gocyclo -over 15 $(GOFMT_FILES)

lint:
	go list $(GOFMT_FILES) | xargs golint 

fmtcheck:
	gofmt -s -l $(GOFMT_FILES)


#migrate:
#	@sh -c "'$(CURDIR)/scripts/db_migrate.sh'"

#docker:
#	@sh -c "'$(CURDIR)/scripts/build.sh'"

.NOTPARALLEL:

.PHONY: bin cover default dev e2etest fmt fmtcheck generate protobuf plugin-dev quickdev test-compile test testacc testrace vendor-status website website-test
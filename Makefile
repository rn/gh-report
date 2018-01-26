GO_COMPILE=linuxkit/go-compile:8235f703735672509a16fb626d25c6ffb0d1c21d

GOOS?=darwin

build: gh-report.go
	docker run -it --rm \
		-v $(CURDIR):/go/src/github.com/rn/utils/gh-report \
		-w /go/src/github.com/rn/utils/gh-report \
		-e GOOS=$(GOOS) \
		--entrypoint go $(GO_COMPILE) build gh-report.go

.PHONY: vendor
vendor:
	docker run -it --rm \
		-v $(CURDIR):/go/src/github.com/rn/utils/gh-report \
		-w /go/src/github.com/rn/utils/gh-report \
		--entrypoint /go/bin/vndr $(GO_COMPILE)

.PHONY: clean
clean:
	rm -f gh-report gh-report.exe

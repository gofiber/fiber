all: build
	git commit -a -m "regenerate web app"

build:
	gobenchdata-web \
		--title "gobenchdata continuous benchmark demo" \
		--source "github.com/bobheadxi/gobenchdata" \
		--canonical-import "go.bobheadxi.dev/gobenchdata" \
		--desc "This is a demo for gobenchdata, a tool and GitHub action for setting up simple continuous benchmarks to monitor performance improvements and regressions in your Golang benchmarks!"

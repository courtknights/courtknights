# Root Makefile — dispatcher

.PHONY: build run test test-int test-all test-cov test-int-cov lint fmt clean \
        db-migrate db-rollback db-status \
        web-install web-start web-build web-test \
        cicd-spec-ref cicd-backend-cov cicd-frontend-cov cicd-adr \
        apply-rulesets tools-test tools-lint

## api targets
build:
	$(MAKE) -C api build

run:
	$(MAKE) -C api run

test:
	$(MAKE) -C api test
	$(MAKE) -C tools/oauthfake test

test-int:
	$(MAKE) -C api test-int

test-all:
	$(MAKE) -C api test-all

test-cov:
	$(MAKE) -C api test-cov COV_OUT=$(COV_OUT)

test-int-cov:
	$(MAKE) -C api test-int-cov COV_OUT=$(COV_OUT)

lint:
	$(MAKE) -C api lint
	$(MAKE) -C tools/oauthfake lint

fmt:
	$(MAKE) -C api fmt

clean:
	$(MAKE) -C api clean

## tools targets
tools-test:
	$(MAKE) -C tools/oauthfake test

tools-lint:
	$(MAKE) -C tools/oauthfake lint

## db targets
db-migrate:
	$(MAKE) -C db migrate

db-rollback:
	$(MAKE) -C db rollback

db-status:
	$(MAKE) -C db status

## cicd targets
cicd-spec-ref:
	$(MAKE) -C infra/cicd check-spec-ref

cicd-backend-cov:
	$(MAKE) -C infra/cicd check-backend-cov

cicd-frontend-cov:
	$(MAKE) -C infra/cicd check-frontend-cov

cicd-adr:
	$(MAKE) -C infra/cicd check-adr

## infra targets
apply-rulesets:
	$(MAKE) -C infra apply-rulesets GITHUB_REPO=$(GITHUB_REPO)

## web targets
web-install:
	$(MAKE) -C web install

web-start:
	$(MAKE) -C web start

web-build:
	$(MAKE) -C web build

web-test:
	$(MAKE) -C web test

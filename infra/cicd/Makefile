# infra/cicd/Makefile — CI check targets
#
# Each target is a standalone check runnable locally by setting the required
# environment variables. See README.md for the full list per target.

.PHONY: check-spec-ref check-backend-cov check-frontend-cov check-adr

check-spec-ref:
	@bash "$(CURDIR)/check_spec_ref.sh"

check-backend-cov:
	@bash "$(CURDIR)/check_backend_coverage.sh"

check-frontend-cov:
	@bash "$(CURDIR)/check_frontend_coverage.sh"

check-adr:
	@echo "not implemented" && exit 0

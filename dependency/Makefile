.PHONY: retrieve

id:
	@echo pnpm

retrieve:
	@cd retrieval; \
	go run . \
		--buildpack_toml_path=$(buildpackTomlPath) \
		--output=$(output)

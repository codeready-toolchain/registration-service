# keycloak-js is the browser adapter for the registration landing page.
# sandbox-dashboard depends on the same release through npm
# (package.json + package-lock.json) and Vite bundles it. This service has no
# Node build, so download that npm package and copy lib/keycloak.js into the
# files picked up by go:embed. CI only needs curl.
KEYCLOAK_JS_VERSION := 26.2.4
# Integrity from sandbox-dashboard/package-lock.json (keycloak-js 26.2.4).
KEYCLOAK_JS_INTEGRITY := sha512-PnXpR3ubETGOt0B/Qt2lxmPbkZr5bc3vlQsOqDoTPPQsZRp7JjhTKxlJ187uWh8qJhvBab6Gsjb06a8ayOPfuw==
KEYCLOAK_JS_URL := https://registry.npmjs.org/keycloak-js/-/keycloak-js-${KEYCLOAK_JS_VERSION}.tgz
KEYCLOAK_JS_FILE := pkg/assets/static/keycloak.js
KEYCLOAK_JS_STAMP := ${OUT_DIR}/keycloak-js.integrity

.PHONY: download-keycloak-js
## Downloads the keycloak-js adapter into the embedded static assets
download-keycloak-js:
	${Q}if [[ -f "${KEYCLOAK_JS_FILE}" && "$$(cat "${KEYCLOAK_JS_STAMP}" 2>/dev/null)" == "${KEYCLOAK_JS_INTEGRITY}" ]]; then \
		exit 0; \
	fi; \
	mkdir -p "${OUT_DIR}"; \
	tmp=$$(mktemp -d "${OUT_DIR}/keycloak-js.XXXXXX"); \
	trap 'rm -rf "$${tmp}"' EXIT; \
	curl -fsSL "${KEYCLOAK_JS_URL}" -o "$${tmp}/keycloak-js.tgz"; \
	expected=$$(printf '%s' "${KEYCLOAK_JS_INTEGRITY}" | sed 's/^sha512-//' | base64 -d | od -An -tx1 | tr -d ' \n'); \
	actual=$$(sha512sum "$${tmp}/keycloak-js.tgz" | awk '{print $$1}'); \
	if [[ "$${expected}" != "$${actual}" ]]; then \
		echo "keycloak-js integrity check failed" >&2; \
		echo "expected $${expected}" >&2; \
		echo "actual   $${actual}" >&2; \
		exit 1; \
	fi; \
	tar -xOf "$${tmp}/keycloak-js.tgz" package/lib/keycloak.js > "${KEYCLOAK_JS_FILE}"; \
	printf '%s\n' "${KEYCLOAK_JS_INTEGRITY}" > "${KEYCLOAK_JS_STAMP}"

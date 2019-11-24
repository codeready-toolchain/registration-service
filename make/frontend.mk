.PHONY: frontend
## builds frontend
frontend:
	cd frontend && pwd && yarn build && cd ../
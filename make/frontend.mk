.PHONY: frontend
## builds frontend for CRTC Landing page

frontend:
	cd frontend && yarn install && yarn build && cd ../
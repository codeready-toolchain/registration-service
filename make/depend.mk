# Only list test and build dependencies
# Standard dependencies are installed via go get
DEPEND=\
	github.com/shurcooL/vfsgen

.PHONY: depend
## install dependencies
depend:
	@echo INSTALLING DEPENDENCIES...
	@env GO111MODULE=off go get -v $(DEPEND)

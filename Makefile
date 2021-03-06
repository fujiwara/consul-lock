GIT_VER := $(shell git describe --tags)

build:
	go get -ldflags "-X main.Version ${GIT_VER}" github.com/fujiwara/consul-lock

binary:
	gox -os="linux darwin windows" -arch="amd64 386" -output "pkg/{{.Dir}}-${GIT_VER}-{{.OS}}-{{.Arch}}" -ldflags "-X main.Version ${GIT_VER}"
	cd pkg && find . -name "*${GIT_VER}*" -type f -exec zip {}.zip {} \;

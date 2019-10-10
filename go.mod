module github.com/codeready-toolchain/registration-service

go 1.13

require (
	github.com/cheekybits/is v0.0.0-20150225183255-68e9c0620927 // indirect
	github.com/codeready-toolchain/api v0.0.0-20190910110833-66e1ab342d1e
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/evanphx/json-patch v4.5.0+incompatible // indirect
	github.com/gin-contrib/gzip v0.0.1
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/gin-gonic/gin v1.4.0
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/spec v0.19.2 // indirect
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/lestrrat-go/jwx v0.9.0
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/matryer/resync v0.0.0-20161211202428-d39c09a11215
	github.com/mattn/go-isatty v0.0.9 // indirect
	github.com/onsi/ginkgo v1.9.0 // indirect
	github.com/onsi/gomega v1.6.0 // indirect
	github.com/operator-framework/operator-sdk v0.10.0
	github.com/pelletier/go-toml v1.4.0 // indirect
	github.com/pkg/errors v0.8.1
	github.com/satori/go.uuid v1.2.0
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v1.4.0
	github.com/stretchr/testify v1.4.0
	github.com/ugorji/go/codec v1.1.7 // indirect
	golang.org/x/crypto v0.0.0-20190907121410-71b5226ff739 // indirect
	golang.org/x/net v0.0.0-20190909003024-a7b16738d86b // indirect
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45 // indirect
	golang.org/x/sys v0.0.0-20190907184412-d223b2b6db03 // indirect
	google.golang.org/appengine v1.6.1 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/square/go-jose.v2 v2.3.1
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.0.0-20190913080256-21721929cffa // indirect
	k8s.io/apimachinery v0.0.0-20190917163033-a891081239f5
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/kube-openapi v0.0.0-20190816220812-743ec37842bf // indirect
	k8s.io/utils v0.0.0-20190809000727-6c36bc71fc4a // indirect
	sigs.k8s.io/controller-runtime v0.1.12
)

// Pinned to kubernetes-1.14.1
replace (
	k8s.io/api => k8s.io/api v0.0.0-20190409021203-6e4e0e4f393b
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190409022649-727a075fdec8
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190409021813-1ec86e4da56c
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190409023024-d644b00f3b79
	k8s.io/client-go => k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20190409023720-1bc0c81fa51d
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20190311093542-50b561225d70
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190409022021-00b8e31abe9d
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20190510232812-a01b7d5d6c22
	k8s.io/kubernetes => k8s.io/kubernetes v1.14.1
)

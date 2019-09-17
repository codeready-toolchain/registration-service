module github.com/codeready-toolchain/registration-service

go 1.12

require (
	github.com/codeready-toolchain/api v0.0.0-20190910110833-66e1ab342d1e
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/evanphx/json-patch v4.5.0+incompatible // indirect
	github.com/gin-contrib/gzip v0.0.1
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/gin-gonic/gin v1.4.0
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/lestrrat-go/jwx v0.9.0
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/mattn/go-isatty v0.0.9 // indirect
	github.com/onsi/ginkgo v1.9.0 // indirect
	github.com/onsi/gomega v1.6.0 // indirect
	github.com/pelletier/go-toml v1.4.0 // indirect
	github.com/pkg/errors v0.8.1
	github.com/satori/go.uuid v1.2.0
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.4.0
	github.com/stretchr/testify v1.4.0
	github.com/ugorji/go v1.1.7 // indirect
	golang.org/x/crypto v0.0.0-20190907121410-71b5226ff739 // indirect
	golang.org/x/net v0.0.0-20190909003024-a7b16738d86b // indirect
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45 // indirect
	golang.org/x/sys v0.0.0-20190907184412-d223b2b6db03 // indirect
	google.golang.org/appengine v1.6.1 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/square/go-jose.v2 v2.3.1
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.0.0-20190820101039-d651a1528133 // indirect
	k8s.io/apimachinery v0.0.0-20190823012420-8ca64af22337
	k8s.io/client-go v0.0.0-20190819141724-e14f31a72a77
	k8s.io/kube-openapi v0.0.0-20190816220812-743ec37842bf // indirect
	k8s.io/utils v0.0.0-20190809000727-6c36bc71fc4a // indirect
)

replace k8s.io/api => k8s.io/api v0.0.0-20181213150558-05914d821849

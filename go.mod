module github.com/codeready-toolchain/registration-service

require (
	github.com/cheekybits/is v0.0.0-20150225183255-68e9c0620927 // indirect
	github.com/codeready-toolchain/api v0.0.0-20210721211719-002c2be44948
	github.com/codeready-toolchain/toolchain-common v0.0.0-20210708074916-046e2bc28f85
	github.com/gin-contrib/cors v1.3.1
	github.com/gin-contrib/gzip v0.0.1
	github.com/gin-gonic/gin v1.7.2
	github.com/go-logr/logr v0.4.0
	github.com/gofrs/uuid v3.3.0+incompatible
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/inconshreveable/log15 v0.0.0-20200109203555-b30bc20e4fd1 // indirect
	github.com/kevinburke/go-types v0.0.0-20200309064045-f2d4aea18a7a // indirect
	github.com/kevinburke/go.uuid v1.2.0 // indirect
	github.com/kevinburke/rest v0.0.0-20200429221318-0d2892b400f8 // indirect
	github.com/kevinburke/twilio-go v0.0.0-20200810163702-320748330fac
	github.com/matryer/resync v0.0.0-20161211202428-d39c09a11215
	github.com/nyaruka/phonenumbers v1.0.57
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.10.0
	github.com/shurcooL/httpfs v0.0.0-20190707220628-8d4bc4ba7749 // indirect
	github.com/shurcooL/vfsgen v0.0.0-20200824052919-0d455de96546
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/ttacon/builder v0.0.0-20170518171403-c099f663e1c2 // indirect
	github.com/ttacon/libphonenumber v1.1.0 // indirect
	gopkg.in/h2non/gock.v1 v1.0.14
	gopkg.in/square/go-jose.v2 v2.3.1
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.3
)

replace github.com/codeready-toolchain/toolchain-common => github.com/rajivnathan/toolchain-common v0.0.0-20210806161817-32064200655e

go 1.16

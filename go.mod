module github.com/codeready-toolchain/registration-service

go 1.18

require (
	github.com/aws/aws-sdk-go v1.44.100
	github.com/codeready-toolchain/api v0.0.0-20221220133801-e9fb0b2352db
	github.com/codeready-toolchain/toolchain-common v0.0.0-20221220133846-3415e8af3654
	github.com/go-logr/logr v1.2.0
	github.com/gofrs/uuid v4.2.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.8.0
	gopkg.in/h2non/gock.v1 v1.0.14
	k8s.io/api v0.24.2
	k8s.io/apimachinery v0.24.2
	k8s.io/apiserver v0.24.2
	k8s.io/client-go v0.24.2
	sigs.k8s.io/controller-runtime v0.12.2
)

require (
	github.com/gin-contrib/cors v1.4.0
	github.com/gin-contrib/gzip v0.0.6
	github.com/gin-contrib/static v0.0.1
	github.com/gin-gonic/gin v1.8.1
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/kevinburke/twilio-go v0.0.0-20220922200631-8f3f155dfe1f
	github.com/matryer/resync v0.0.0-20161211202428-d39c09a11215
	github.com/nyaruka/phonenumbers v1.1.1
	github.com/spf13/pflag v1.0.5
	go.uber.org/zap v1.19.1
	gopkg.in/square/go-jose.v2 v2.3.0
	gotest.tools v2.2.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.60.1
)

require k8s.io/utils v0.0.0-20220210201930-3a6ce19ff2f9 // indirect

require (
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/cheekybits/is v0.0.0-20150225183255-68e9c0620927 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful v2.12.0+incompatible // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-logr/zapr v1.2.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.19.5 // indirect
	github.com/go-openapi/swag v0.19.14 // indirect
	github.com/go-playground/locales v0.14.0 // indirect
	github.com/go-playground/universal-translator v0.18.0 // indirect
	github.com/go-playground/validator/v10 v10.10.0 // indirect
	github.com/goccy/go-json v0.9.7 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/go-cmp v0.5.7 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/h2non/parth v0.0.0-20190131123155-b4df798d6542 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kevinburke/go-types v0.0.0-20210723172823-2deba1f80ba7 // indirect
	github.com/kevinburke/rest v0.0.0-20210506044642-5611499aa33c // indirect
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/lestrrat-go/jwx v0.9.2 // indirect
	github.com/mailru/easyjson v0.7.6 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	// using latest commit from 'github.com/openshift/api branch release-4.11'
	github.com/openshift/api v0.0.0-20220912161038-458ad9ca9ca5 // indirect
	github.com/pelletier/go-toml/v2 v2.0.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/ttacon/builder v0.0.0-20170518171403-c099f663e1c2 // indirect
	github.com/ttacon/libphonenumber v1.2.1 // indirect
	github.com/ugorji/go/codec v1.2.7 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/crypto v0.0.0-20220214200702-86341886e292 // indirect
	golang.org/x/net v0.0.0-20220614195744-fb05da6f9022 // indirect
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8 // indirect
	golang.org/x/sync v0.0.0-20220601150217-0de741cfad7f // indirect
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20220210224613-90d013bbcef8 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/kube-openapi v0.0.0-20220328201542-3ee0da9b0b42 // indirect
	sigs.k8s.io/json v0.0.0-20211208200746-9f7c6b3444d2 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

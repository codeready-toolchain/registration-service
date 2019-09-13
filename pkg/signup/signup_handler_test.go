package signup_test

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignupHandler(t *testing.T) {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodPost, "/api/v1/signup", nil)
	require.NoError(t, err)

	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)
	configRegistry := configuration.CreateEmptyRegistry()

	// Set the config for testing mode, the handler may use this.
	configRegistry.GetViperInstance().Set("testingmode", true)
	assert.True(t, configRegistry.IsTestingMode(), "testing mode not set correctly to true")

	// Create handler instance.
	signupService := signup.NewSignupService(logger, configRegistry)
	handler := gin.HandlerFunc(signupService.PostSignupHandler)

	t.Run("signup", func(t *testing.T) {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(t, http.StatusOK, rr.Code)
	})
}

// Override time value for tests.  Restore default value after.
func at(t time.Time, f func()) {
	jwt.TimeFunc = func() time.Time {
		return t
	}
	f()
	jwt.TimeFunc = time.Now
}

// TokenClaims represents access token claims
type TokenClaims struct {
	Name          string         `json:"name"`
	Username      string         `json:"preferred_username"`
	GivenName     string         `json:"given_name"`
	FamilyName    string         `json:"family_name"`
	Email         string         `json:"email"`
	EmailVerified bool           `json:"email_verified"`
	Company       string         `json:"company"`
	SessionState  string         `json:"session_state"`
	Approved      bool           `json:"approved"`
	Permissions   *[]Permissions `json:"permissions"`
	jwt.StandardClaims
}

// Permissions represents a "permissions" claim in the AuthorizationPayload
type Permissions struct {
	ResourceSetName *string  `json:"resource_set_name"`
	ResourceSetID   *string  `json:"resource_set_id"`
	Scopes          []string `json:"scopes"`
	Expiry          int64    `json:"exp"`
}

func TestToken(t *testing.T) {

	  tokenString := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJmb28iOiJiYXIiLCJleHAiOjE1MDAwLCJpc3MiOiJ0ZXN0In0.HE7fK0xOQwFEr4WDgRWj4teRPZ6i3GLwD5YCm6Pwu_c"
		
		type MyCustomClaims struct {
			Foo string `json:"foo"`
			jwt.StandardClaims
		}

		// sample token is expired.  override time so it parses as valid
		at(time.Unix(0, 0), func() {
			token, err := jwt.ParseWithClaims(tokenString, &MyCustomClaims{}, func(token *jwt.Token) (interface{}, error) {
				return []byte("AllYourBase"), nil
			})

			if claims, ok := token.Claims.(*MyCustomClaims); ok && token.Valid {
				fmt.Printf("%v %v", claims.Foo, claims.StandardClaims.ExpiresAt)
			} else {
				fmt.Println(err)
			}
		})

	/*

	tokenString := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6InRlc3Qta2V5In0.eyJqdGkiOiIwMjgyYjI5Yy01MTczLTQyZDgtODE0NS1iNDVmYTFlMzUzOGIiLCJleHAiOjAsIm5iZiI6MCwiaWF0IjoxNTE3MDE1OTUyLCJpc3MiOiJ0ZXN0IiwiYXVkIjoiZmFicmljOC1vbmxpbmUtcGxhdGZvcm0iLCJzdWIiOiIyMzk4NDM5OC04NTVhLTQyZDYtYTdmZS05MzZiYjRlOTJhMGMiLCJ0eXAiOiJCZWFyZXIiLCJzZXNzaW9uX3N0YXRlIjoiZWFkYzA2NmMtMTIzNC00YTU2LTlmMzUtY2U3MDdiNTdhNGU5IiwiYWNyIjoiMCIsImFsbG93ZWQtb3JpZ2lucyI6WyIqIl0sImFwcHJvdmVkIjp0cnVlLCJlbWFpbF92ZXJpZmllZCI6dHJ1ZSwibmFtZSI6IlRlc3QiLCJjb21wYW55IjoiIiwicHJlZmVycmVkX3VzZXJuYW1lIjoidGVzdHVzZXIiLCJnaXZlbl9uYW1lIjoiIiwiZmFtaWx5X25hbWUiOiIiLCJlbWFpbCI6InRAdGVzdC50In0.gyoMIWuXnIMMRHewef-__Wkd66qjqSSJxusWcFVtNWaYOXWu7iFV9DhtPVGsbTllXG_lDozPV9BaDmmYRotnn3ZBg7khFDykv9WnoYAjE9vW1d8szNjuoG3tfgQI4Dr9jqopSLndldxq97LGqpxqZFbIDlYd8vN47kv4EePOZDsII6egkTraCMc35eMMilJ4Udd6CMqyV_zaYiGhgAGgeL2ovMFhg_jnc7WhePv7FZkUmtfhCuLUL2TSXS6CyWZYoUDEIcfca6cMzuKOzJoONkDJShNo4u_cQ53duXX_bizdwYNlzBHfIPhSR1LDgV9BXoM6YQnw3It8ReCfF8BEMQ"


	devModeRsaPrivateKey := `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA40yB6SNoU4SpWxTfG5ilu+BlLYikRyyEcJIGg//w/GyqtjvT
/CVo92DRTh/DlrgwjSitmZrhauBnrCOoUBMin0/TXeSo3w2M5tEiiIFPbTDRf2jM
fbSGEOke9O0USCCR+bM2TncrgZR74qlSwq38VCND4zHc89rAzqJ2LVM2aXkuBbO7
TcgLNyooBrpOK9khVHAD64cyODAdJY4esUjcLdlcB7TMDGOgxGGn2RARU7+TUf32
gZZbTMikbuPM5gXuzGlo/22ECbQSKuZpbGwgPIAZ5NN9QA4D1NRz9+KDoiXZ6deZ
TTVCrZykJJ6RyLNfRh+XS+6G5nvcqAmfBpyOWwIDAQABAoIBAE5pBie23zZwfTu+
Z3jNn96/+idLC+DBqq5qsXS3xhpOIlXbLbW98gfkjk+1BXPo9la7wadLlpeX8iuf
4WA+OaNblj69ssO/mOvHGXKdqRixzpN1Q5XZwKX0xYkYf/ahxbmt6P4IfimlX1dB
shsWigU8ZR7rBJ3ayMh/ouTf39ViIbXsHYpEubmACcLaOlXbEuZNr7ofkFQKl/mh
XLWUeOoM97xY6Agw/gv60GIcxIC5OAg7iNqS+XNzhba7f2nf2YqodbN9H1BmEJsf
RRaTTWlZAiQXC8lpZOKwP7DiMLOT78lfmlYtquEBhwRbXazfzsdf67Mr4Kdl2Cej
Jy0EGwECgYEA/DZWB0Lb0tPdT1FmORNrBfGg3PjhX9FOilhbtUgX3nNKp8Zsi3yO
yN6hf0/98qIGlmAQi5C92cXpdhqTiVAGktWD+q0a1W99udIjinS1tFrKgNtOyBWN
uwDBZyhw8RrwpQinMe7B966SVDaphvvOWlB1TadMDh5kReJCYpvRCrMCgYEA5rZj
djCU2UqMw6jIP07nCFjWgxPPjg7jP8aRo07oW2mv1sEA0doCyoZaMrdNeGd3fB0B
sm+IvlQtWD7r0tWZI1GkYpdRkDFurdkIzVPV5pMwH4ByOq/Jf5ZqtjIpoMaRBirA
whJyjmiGU3yDyPDLtEFpNgqM3mIyxS6M6UGKYbkCgYEAg6w+d6YBK+1uQiXGD5BC
tKS0jgjlaOfWcEW3A0qzI3Dfjf3610vdI6OPfu8dLppGhCV9HdAgPdykiQNQ+UQt
WmVcdPgA5WNCqUu7QGK0Joer52AXnkAacYHwdtHXPRkKf66n01rKK2wZexvan91A
m0gcJcFs5IYbZZy9ecvNdB8CgYEAo4JZ5Vay93j1YGnLWcrixDCp/wXYUJbOidGC
QBpZZQf3Hh11JkT7O2uSm2T727yAmw63uC2B3VotNOCLI8ZMHRLsjQ8vOCFAjqdF
rLeg3iQss/bFfkA9b1Y8VNoiVJbGC3fbWu/WDoWXxa12fL/jruG43hsGEUnJL6Q5
K8tOdskCgYABpoHFRxsvJ5Sp9CUS3BBTicVSkpAjoX2O3+cS9XL8IsIqZEMW7VKb
16/H2BRvI0uUq12t+UCc0P0SyrWRGxwGR5zSYHVDOot5EDHqE8aYSbX4jiXtAAiu
qCn3Rug8QWyBjjxnU3CxPRiLSmEllQAAVlzfRWn6kL4RKSyruUhZaA==
-----END RSA PRIVATE KEY-----`

	rsaKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(devModeRsaPrivateKey))
	if err != nil {
		log.Println("error parsing pem")
	}
	publicKey := &rsaKey.PublicKey

	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		kid := token.Header["kid"]
		if kid == nil {
			log.Println("There is no 'kid' header in the token")
			return nil, errors.New("There is no 'kid' header in the token")
		}
		// get the public key for kid
		log.Println("kid ==", kid)
		return publicKey, nil
	})

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		fmt.Printf("%v %v", claims.Username, claims.StandardClaims.ExpiresAt)
	} else {
		fmt.Println(err)
	}
*/
}

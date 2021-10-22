package fake

import (
	"testing"

	"gopkg.in/h2non/gock.v1"
)

var certsContent = `{"keys":[{"kid":"E3DKGdZQ7xTiIvfdFgVXLNupVupFBlcxNUgVCFhDwEg","kty":"RSA","alg":"RS512","use":"sig","n":"ta1xAjqdqnH_RlDI1rFtiGWYgnxpzqGflSQXzuiKR1QaipHTeGeLDUTcG1O6nlb9YgEVcJKSP8JQ36QNfXCPKlNcsqUqr81jiL_kSNAD3xHX4Z8ymuA-FW24bLeNwRkdGKGy3aY4giJxXnqB63ArtjmmWaGYEQEriUz16wW0w3H_QJyje3__j_Sh1ya_V7Ct3A6ajTipp-OzAuIgsqXbZz2b8ejr3My5PiXz9t41xKx_u4Mm18BQ4SQ2OvTfA0Of0mZ3Q-FVy2q1WIKwPmCMDyV5bigmvRYblRDCbTvKIGHyEjs1zuAxJqzFJkGpAHpnKfbUdSfO-JWK6fB4V3bPzw","e":"AQAB","x5c":["MIICrTCCAZUCBgF3qV4+jzANBgkqhkiG9w0BAQsFADAaMRgwFgYDVQQDDA9yZWRoYXQtZXh0ZXJuYWwwHhcNMjEwMjE2MDU0MjQxWhcNMzEwMjE2MDU0NDIxWjAaMRgwFgYDVQQDDA9yZWRoYXQtZXh0ZXJuYWwwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC1rXECOp2qcf9GUMjWsW2IZZiCfGnOoZ+VJBfO6IpHVBqKkdN4Z4sNRNwbU7qeVv1iARVwkpI/wlDfpA19cI8qU1yypSqvzWOIv+RI0APfEdfhnzKa4D4Vbbhst43BGR0YobLdpjiCInFeeoHrcCu2OaZZoZgRASuJTPXrBbTDcf9AnKN7f/+P9KHXJr9XsK3cDpqNOKmn47MC4iCypdtnPZvx6OvczLk+JfP23jXErH+7gybXwFDhJDY69N8DQ5/SZndD4VXLarVYgrA+YIwPJXluKCa9FhuVEMJtO8ogYfISOzXO4DEmrMUmQakAemcp9tR1J874lYrp8HhXds/PAgMBAAEwDQYJKoZIhvcNAQELBQADggEBALWRXIDVRxEux7UleQbyuA8+jdTRzhScTBiL24NHzRofg5jcWjhCyGxitrhp16sC7+lEQaPTcNGmJIk0uVtExGm6N1WG653Ubkq+KaiQiJPFELZS31x7xLAUo7aNHPVbS6Rr4ufUiFcT2cS0e7sjVlf9FvtX9fdg1TSpq52Vaayz4RXYCx+IrHEmU0L5qDJPyHiuBJ8VvnkcQMqYZ5aAA1z0/HSsF7AIraeyPbQANfJSuvFIPR0+fk/pcvUMB/XMk3obMXYzUMAa4BcOnVcmymcNc8Tf5kwqDIy6Y03yIVRrvKX5aPsBRqAzUtNE4rLkPqhBV+U0dR/xFiLDn3cGyjk="],"x5t":"ZHBbdjfzncqH7ewCO4h6h0HKCUM","x5t#S256":"j0wxZVV5frSC2rs_Kg6cK8RSwDKXMUMwSPPqd3XCO6c"}]}`

func MockKeycloakCertsCall(t *testing.T) {
	gock.New("https://sso.devsandbox.dev").
		Get("auth/realms/sandbox-dev/protocol/openid-connect/certs").
		Persist().
		Reply(200).
		BodyString(certsContent)
	t.Cleanup(gock.OffAll)
}

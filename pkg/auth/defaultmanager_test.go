package auth

import (
	"sync"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/codeready-toolchain/registration-service/test/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestDefaultManagerSuite struct {
	test.UnitTestSuite
}

func TestRunDefaultManagerSuite(t *testing.T) {
	suite.Run(t, &TestDefaultManagerSuite{test.UnitTestSuite{}})
}

func (s *TestDefaultManagerSuite) TestKeyManagerDefaultTokenParser() {
	// reset the singletons
	defaultTokenParser = nil
	initDefaultTokenParserOnce = &sync.Once{}

	fake.MockKeycloakCertsCall(s.T())

	// Set the config for testing mode, the handler may use this.
	assert.True(s.T(), configuration.IsTestingMode(), "testing mode not set correctly to true")

	s.Run("get before init", func() {
		_, err := DefaultTokenParser()
		require.Error(s.T(), err)
		require.Equal(s.T(), "no default TokenParser created, call `InitializeDefaultTokenParser()` first", err.Error())
	})

	s.Run("multiple initialization", func() {
		p1, err := InitializeDefaultTokenParser()
		require.NoError(s.T(), err)

		p2, err := InitializeDefaultTokenParser()
		require.NoError(s.T(), err)

		// Second initialization should return the same parser from the first initialization
		require.Same(s.T(), p1, p2)
	})

	s.Run("retrieval", func() {
		_, err := DefaultTokenParser()
		require.NoError(s.T(), err)
	})

	s.Run("parallel threads", func() {
		// reset the singletons
		defaultTokenParser = nil
		initDefaultTokenParserOnce = new(sync.Once)
		type tpErrHolder struct {
			TokePrsr *TokenParser
			TpErr    error
		}

		latch := sync.WaitGroup{}
		latch.Add(1)
		holder := make([]*tpErrHolder, 100)
		wg := sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				// now, wait for latch to be released so that all workers start at the same time
				latch.Wait()
				tp, err := InitializeDefaultTokenParser()
				thisHolder := &tpErrHolder{
					TokePrsr: tp,
					TpErr:    err,
				}
				holder[i] = thisHolder
			}(i)
		}
		latch.Done()
		// wait for the system to settle before checking the results
		wg.Wait()

		require.NotNil(s.T(), holder[0].TokePrsr)
		// check that all entries have a TokenParser
		for _, entry := range holder {
			require.NoError(s.T(), entry.TpErr)
			require.Same(s.T(), holder[0].TokePrsr, entry.TokePrsr)
		}
	})
}

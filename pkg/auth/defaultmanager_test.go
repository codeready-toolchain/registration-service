package auth

import (
	"log"
	"os"
	"sync"
	"testing"
	"time"

	testutils "github.com/codeready-toolchain/registration-service/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestDefaultManagerSuite struct {
	testutils.UnitTestSuite
}

func TestRunDefaultManagerSuite(t *testing.T) {
	suite.Run(t, &TestDefaultManagerSuite{testutils.UnitTestSuite{}})
}

func (s *TestDefaultManagerSuite) TestKeyManagerDefaultKeyManager() {
	// reset the singletons
	defaultKeyManagerHolder = nil
	defaultTokenParserHolder = nil

	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)

	// Set the config for testing mode, the handler may use this.
	assert.True(s.T(), s.Config.IsTestingMode(), "testing mode not set correctly to true")

	s.Run("get before init", func() {
		_, err := defaultKeyManager()
		require.Error(s.T(), err)
		require.Equal(s.T(), "no default KeyManager created, call `InitializeDefaultKeyManager()` first", err.Error())
	})

	s.Run("first creation", func() {
		_, err := initializeDefaultKeyManager(logger, s.Config)
		require.NoError(s.T(), err)
	})

	s.Run("second redundant creation", func() {
		_, err := initializeDefaultKeyManager(logger, s.Config)
		require.Error(s.T(), err)
		require.Equal(s.T(), "default KeyManager can be created only once", err.Error())
	})

	s.Run("retrieval", func() {
		_, err := defaultKeyManager()
		require.NoError(s.T(), err)
	})

	s.Run("parallel threads", func() {
		// reset the singleton
		defaultKeyManagerHolder = nil
		defaultTokenParserHolder = nil
		type kmErrHolder struct {
			KeyMngr *KeyManager
			KmErr   error
		}
		latch := sync.WaitGroup{}
		latch.Add(1)
		holder := make([]*kmErrHolder, 3)
		for i := 0; i < 3; i++ {
			go func(i int) {
				// now, wait for latch to be released so that all workers start at the same time
				latch.Wait()
				km, err := initializeDefaultKeyManager(logger, s.Config)
				thisHolder := &kmErrHolder{
					KeyMngr: km,
					KmErr:   err,
				}
				holder[i] = thisHolder
			}(i)
		}
		latch.Done()
		// wait for the system to settle before checking the results
		time.Sleep(time.Millisecond * 500)
		// check if only one entry has a KeyManager and the two others have errs
		fails := 0
		success := 0
		for i := 0; i < 3; i++ {
			thisEntry := holder[i]
			if thisEntry.KeyMngr != nil && thisEntry.KmErr == nil {
				success++
			}
			if thisEntry.KeyMngr == nil && thisEntry.KmErr != nil {
				fails++
			}
			if (thisEntry.KeyMngr == nil && thisEntry.KmErr == nil) || (thisEntry.KeyMngr != nil && thisEntry.KmErr != nil) {
				require.Fail(s.T(), "unexpected return values when calling InitializeDefaultKeyManager")
			}
		}
		require.Equal(s.T(), 1, success)
		require.Equal(s.T(), 2, fails)
	})
}

func (s *TestDefaultManagerSuite) TestKeyManagerDefaultTokenParser() {
	// reset the singletons
	defaultKeyManagerHolder = nil
	defaultTokenParserHolder = nil

	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)

	// Set the config for testing mode, the handler may use this.
	assert.True(s.T(), s.Config.IsTestingMode(), "testing mode not set correctly to true")

	s.Run("get before init", func() {
		_, err := DefaultTokenParser()
		require.Error(s.T(), err)
		require.Equal(s.T(), "no default TokenParser created, call `InitializeDefaultTokenParser()` first", err.Error())
	})

	s.Run("first creation", func() {
		_, err := InitializeDefaultTokenParser(logger, s.Config)
		require.NoError(s.T(), err)
	})

	s.Run("second redundant creation", func() {
		_, err := InitializeDefaultTokenParser(logger, s.Config)
		require.Error(s.T(), err)
		require.Equal(s.T(), "default TokenParser can be created only once", err.Error())
	})

	s.Run("retrieval", func() {
		_, err := DefaultTokenParser()
		require.NoError(s.T(), err)
	})

	s.Run("parallel threads", func() {
		// reset the singletons
		defaultKeyManagerHolder = nil
		defaultTokenParserHolder = nil
		type tpErrHolder struct {
			TokePrsr *TokenParser
			TpErr    error
		}
		latch := sync.WaitGroup{}
		latch.Add(1)
		holder := make([]*tpErrHolder, 3)
		for i := 0; i < 3; i++ {
			go func(i int) {
				// now, wait for latch to be released so that all workers start at the same time
				latch.Wait()
				tp, err := InitializeDefaultTokenParser(logger, s.Config)
				thisHolder := &tpErrHolder{
					TokePrsr: tp,
					TpErr:    err,
				}
				holder[i] = thisHolder
			}(i)
		}
		latch.Done()
		// wait for the system to settle before checking the results
		time.Sleep(time.Millisecond * 500)
		// check if only one entry has a TokenParser and the two others have errs
		fails := 0
		success := 0
		for i := 0; i < 3; i++ {
			thisEntry := holder[i]
			if thisEntry.TokePrsr != nil && thisEntry.TpErr == nil {
				success++
			}
			if thisEntry.TokePrsr == nil && thisEntry.TpErr != nil {
				fails++
			}
			if (thisEntry.TokePrsr == nil && thisEntry.TpErr == nil) || (thisEntry.TokePrsr != nil && thisEntry.TpErr != nil) {
				require.Fail(s.T(), "unexpected return values when calling InitializeDefaultTokenParser")
			}
		}
		require.Equal(s.T(), 1, success)
		require.Equal(s.T(), 2, fails)
	})
}

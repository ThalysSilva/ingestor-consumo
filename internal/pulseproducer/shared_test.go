package pulseproducer

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
)

func TestMain(m *testing.M) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	originalMarshalFunc := marshalFunc
	defer func() { marshalFunc = originalMarshalFunc }()

	originalUUIDFunc := uuidFunc
	defer func() { uuidFunc = originalUUIDFunc }()

	exitCode := m.Run()

	os.Exit(exitCode)

}

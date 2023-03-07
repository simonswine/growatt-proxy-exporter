package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsing(t *testing.T) {
	var (
		r Msg
		d Data
	)

	for _, filePath := range []string{
		// NOTE: these files are not included in the repo, as they need to be cleared of identifying information first
		"testdata/realdata1.bin",
		"testdata/realdata2.bin",
	} {
		t.Run(filepath.Base(filePath), func(t *testing.T) {

			in, err := os.ReadFile(filePath)
			require.NoError(t, err)

			// decode the first layer
			require.NoError(t, decode(in, &r))

			assert.Equal(t, uint16(6), r.ProtocolID)
			assert.Equal(t, uint16(831), r.Length)
			assert.Equal(t, r.Length, uint16(len(r.Payload)+2))
			assert.Equal(t, uint8(0x1), r.UnitID)
			assert.Equal(t, MsgTypeData, r.Type)

			// decode the data message
			require.NoError(t, decodeDataMessage(&r, &d))

			// compare the data message to the expected output
			exp, err := os.ReadFile(strings.ReplaceAll(filePath, ".bin", ".json"))
			require.NoError(t, err)

			act, err := json.MarshalIndent(d, "", "  ")
			require.NoError(t, err)

			assert.JSONEq(t, string(exp), string(act))
		})
	}
}

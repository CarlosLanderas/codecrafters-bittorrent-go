package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type DecoderCase struct {
	value   string
	decoded interface{}
	valid   bool
	st      int
}

func Test_Decode_String(t *testing.T) {

	testCases := []DecoderCase{
		{
			value:   "4:fish",
			decoded: "fish",
			valid:   true,
			st:      6,
		},
		{
			value:   "10:murcielago",
			decoded: "murcielago",
			valid:   true,
			st:      13,
		},
		{
			value: "invalid",
			valid: false,
		},
		{
			value: "14:exceeded",
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			decoded, pos, err := decodeString(tc.value, 0)

			if !tc.valid {
				assert.Error(t, err)
			} else {
				assert.Equal(t, decoded, tc.decoded)
				assert.Equal(t, tc.st, pos)
			}
		})
	}
}

func Test_Decode_Int(t *testing.T) {
	testCases := []DecoderCase{
		{
			value:   "i60e",
			decoded: 60,
			valid:   true,
			st:      4,
		},
		{
			value:   "i-452e",
			decoded: -452,
			valid:   true,
			st:      6,
		},
		{
			value: "i34",
			valid: false,
		},
	}

	for _, tc := range testCases {
		decoded, pos, err := decodeInt(tc.value, 0)

		if !tc.valid {
			assert.Error(t, err)
		} else {
			assert.Equal(t, decoded, tc.decoded)
			assert.Equal(t, tc.st, pos)
		}
	}
}

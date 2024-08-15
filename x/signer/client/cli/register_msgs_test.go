package cli

import (
	"errors"
	"reflect"
	"testing"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestSanitizeSymbolName(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty string", "", ""},
		{"Simple type URL", "/cosmos.bank.v1beta1.MsgSend", "Cosmos_bank_v1beta1_MsgSend"},
		{"Type URL with multiple dots", "/cosmos.bank.v1.beta1.MsgSend", "Cosmos_bank_v1_beta1_MsgSend"},
		{"Type URL with uppercase letters", "/cosmos.Bank.v1beta1.MsgSend", "Cosmos_Bank_v1beta1_MsgSend"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeSymbolName(tc.input)
			if result != tc.expected {
				t.Errorf("Expected: %s, Got: %s", tc.expected, result)
			}
		})
	}
}

func TestCapitalizeFirstChar(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty string", "", ""},
		{"Single lowercase letter", "a", "A"},
		{"Single uppercase letter", "A", "A"},
		{"Multiple letters", "hello", "Hello"},
		{"All uppercase letters", "WORLD", "WORLD"},
		{"Mixed case", "hEllO", "HEllO"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := capitalizeFirstChar(tc.input)
			if result != tc.expected {
				t.Errorf("Expected: %s, Got: %s", tc.expected, result)
			}
		})
	}
}

func TestGetLookupPackages(t *testing.T) {
	testCases := []struct {
		name          string
		unregistered  map[string]struct{}
		expectedPaths map[string]struct{}
	}{
		{
			name:          "Empty unregistered types",
			unregistered:  map[string]struct{}{},
			expectedPaths: map[string]struct{}{},
		},
		{
			name: "Single type URL",
			unregistered: map[string]struct{}{
				"/cosmos.bank.v1beta1.MsgSend": {},
			},
			expectedPaths: map[string]struct{}{
				"/cosmos.bank.v1beta1": {},
			},
		},
		{
			name: "Multiple type URLs with same prefix",
			unregistered: map[string]struct{}{
				"/cosmos.bank.v1beta1.MsgSend":    {},
				"/cosmos.bank.v1beta1.MsgDeposit": {},
			},
			expectedPaths: map[string]struct{}{
				"/cosmos.bank.v1beta1": {},
			},
		},
		{
			name: "Multiple type URLs with different prefixes",
			unregistered: map[string]struct{}{
				"/cosmos.bank.v1beta1.MsgSend":        {},
				"/cosmos.staking.v1beta1.MsgDelegate": {},
			},
			expectedPaths: map[string]struct{}{
				"/cosmos.bank.v1beta1":    {},
				"/cosmos.staking.v1beta1": {},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getLookupPackages(tc.unregistered)
			if !reflect.DeepEqual(result, tc.expectedPaths) {
				t.Errorf("Expected lookup paths: %v, Got: %v", tc.expectedPaths, result)
			}
		})
	}
}

func TestFindUnregisteredTypes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock InterfaceRegistry that implements the types.InterfaceRegistry interface
	mIRegistry := NewMockInterfaceRegistry(ctrl)

	// Setup the expected method calls and return values for the mock InterfaceRegistry
	mIRegistry.EXPECT().Resolve("/cosmos.bank.v1beta1.MsgSend").Return(nil, nil).AnyTimes()                               // Registered type
	mIRegistry.EXPECT().Resolve("/cosmos.bank.v1beta1.MsgBurn").Return(nil, errors.New("type not registered")).AnyTimes() // Unregistered type

	// Create a mock Codec
	mCodec := NewMockCodec(ctrl)

	// Setup the expected method calls and return values for the mock Codec
	mCodec.EXPECT().InterfaceRegistry().Return(mIRegistry).AnyTimes()

	clientCtx := client.Context{
		Codec: mCodec,
	}

	tests := []struct {
		name           string
		messages       []map[string]any
		expectedResult map[string]struct{}
		expectedError  bool
	}{
		{
			name: "No unregistered types",
			messages: []map[string]any{
				{"@type": "/cosmos.bank.v1beta1.MsgSend"},
			},
			expectedResult: map[string]struct{}{}, // No unregistered types expected
			expectedError:  false,
		},
		{
			name: "One unregistered type",
			messages: []map[string]any{
				{"@type": "/cosmos.bank.v1beta1.MsgBurn"},
			},
			expectedResult: map[string]struct{}{
				"/cosmos.bank.v1beta1.MsgBurn": {}, // Unregistered type expected
			},
			expectedError: false,
		},
		{
			name: "Nested messages with unregistered types",
			messages: []map[string]any{
				{
					"msg": []map[string]any{
						{"@type": "/cosmos.bank.v1beta1.MsgBurn"},
						{"@type": "/cosmos.bank.v1beta1.MsgSend"},
					},
				},
			},
			expectedResult: map[string]struct{}{
				"/cosmos.bank.v1beta1.MsgBurn": {}, // Only the unregistered type expected
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := findUnregisteredTypes(clientCtx, tt.messages)
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedResult, result)
		})
	}
}

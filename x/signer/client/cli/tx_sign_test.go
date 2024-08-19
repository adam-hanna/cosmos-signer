package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"cosmossdk.io/core/address"
	"cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	rpcclientmock "github.com/cometbft/cometbft/rpc/client/mock"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	sdk "github.com/cosmos/cosmos-sdk/types"
	testutilmod "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/gov"
	"github.com/stretchr/testify/suite"
)

func TxSignExec(clientCtx client.Context, from sdk.AccAddress, filename string, extraArgs ...string) (testutil.BufferWriter, error) {
	//account, err := clientCtx.AccountRetriever.GetAccount(clientCtx, from)
	//if err != nil {
	//	return nil, err
	//}
	//if account == nil {
	//	return nil, fmt.Errorf("account not found: %s", from)
	//}
	fmt.Printf("keyring dir: %s\n", clientCtx.KeyringDir)
	args := []string{
		fmt.Sprintf("--from=%s", from.String()),
		fmt.Sprintf("--%s=%s", flags.FlagHome, strings.Replace(clientCtx.HomeDir, "simd", "simcli", 1)),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, clientCtx.ChainID),
		fmt.Sprintf("--keyring-backend=%s", clientCtx.Keyring.Backend()),
		"--keyring-dir=.",
		"--plugins-dir=./testfiles/",
		//fmt.Sprintf("-a=%d", account.GetAccountNumber()),
		//fmt.Sprintf("--sequence=%d", account.GetSequence()),
		"-a=0",
		"--sequence=2",
		"--offline",
		filename,
	}

	cmd := GetSignCommand()
	cmd.PersistentFlags().String(flags.FlagHome, clientCtx.HomeDir, "directory for config and data")

	return clitestutil.ExecTestCLICmd(clientCtx, cmd, append(args, extraArgs...))
}

func TxValidateSignaturesExec(clientCtx client.Context, from sdk.AccAddress, filename string) (testutil.BufferWriter, error) {
	//account, err := clientCtx.AccountRetriever.GetAccount(clientCtx, from)
	//if err != nil {
	//	return nil, err
	//}
	args := []string{
		filename,
		// fmt.Sprintf("--%s=%s", flags.FlagHome, strings.Replace(clientCtx.HomeDir, "simd", "simcli", 1)),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, clientCtx.ChainID),
		"--offline",
		fmt.Sprintf("--keyring-backend=%s", clientCtx.Keyring.Backend()),
		"--keyring-dir=.",
		"--plugins-dir=./testfiles/",
		//fmt.Sprintf("-a=%d", account.GetAccountNumber()),
		//fmt.Sprintf("--sequence=%d", account.GetSequence()),
		"-a=0",
		"--sequence=2",
	}

	return clitestutil.ExecTestCLICmd(clientCtx, GetValidateSignaturesCommand(), args)
}

type CLITestSuite struct {
	suite.Suite

	kr        keyring.Keyring
	encCfg    testutilmod.TestEncodingConfig
	baseCtx   client.Context
	clientCtx client.Context
	val       sdk.AccAddress
	val1      sdk.AccAddress

	ac address.Codec
}

func (s *CLITestSuite) createBankMsg(clientCtx client.Context, toAddr sdk.AccAddress, amount sdk.Coins, extraFlags ...string) (testutil.BufferWriter, error) {
	flags := []string{
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastSync),
		fmt.Sprintf("--%s=%s", flags.FlagFees,
			sdk.NewCoins(sdk.NewCoin("stake", math.NewInt(10))).String()),
	}

	flags = append(flags, extraFlags...)
	return clitestutil.MsgSendExec(clientCtx, s.val, toAddr, amount, s.ac, flags...)
}

func TestCLITestSuite(t *testing.T) {
	suite.Run(t, new(CLITestSuite))
}

func (s *CLITestSuite) TearDownSuite() {
	os.RemoveAll("./keyring-test")
}

func (s *CLITestSuite) SetupSuite() {
	s.encCfg = testutilmod.MakeTestEncodingConfig(auth.AppModuleBasic{}, bank.AppModuleBasic{}, gov.AppModuleBasic{})
	//tmpDir, err := os.MkdirTemp("", fmt.Sprintf("tmpKeyringDir-%s", time.Now().Format("20060102-150405")))
	//s.Require().NoError(err)
	kr, err := keyring.New("test", keyring.BackendTest, ".", nil, s.encCfg.Codec)
	s.Require().NoError(err)
	s.kr = kr
	// s.kr = keyring.NewInMemory(s.encCfg.Codec)
	s.baseCtx = client.Context{}.
		WithKeyring(s.kr).
		WithTxConfig(s.encCfg.TxConfig).
		WithCodec(s.encCfg.Codec).
		WithClient(clitestutil.MockCometRPC{Client: rpcclientmock.Client{}}).
		WithAccountRetriever(client.MockAccountRetriever{}).
		WithOutput(io.Discard).
		WithChainID("test-chain")

	ctxGen := func() client.Context {
		bz, _ := s.encCfg.Codec.Marshal(&sdk.TxResponse{})
		c := clitestutil.NewMockCometRPC(abci.ResponseQuery{
			Value: bz,
		})
		return s.baseCtx.WithClient(c)
	}
	s.clientCtx = ctxGen()

	kb := s.clientCtx.Keyring
	valAcc, _, err := kb.NewMnemonic("newAccount", keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	s.Require().NoError(err)
	s.val, err = valAcc.GetAddress()
	s.Require().NoError(err)

	account1, _, err := kb.NewMnemonic("newAccount1", keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	s.Require().NoError(err)
	s.val1, err = account1.GetAddress()
	s.Require().NoError(err)

	account2, _, err := kb.NewMnemonic("newAccount2", keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	s.Require().NoError(err)
	pub1, err := account1.GetPubKey()
	s.Require().NoError(err)
	pub2, err := account2.GetPubKey()
	s.Require().NoError(err)

	// Create a dummy account for testing purpose
	_, _, err = kb.NewMnemonic("dummyAccount", keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	s.Require().NoError(err)

	multi := kmultisig.NewLegacyAminoPubKey(2, []cryptotypes.PubKey{pub1, pub2})
	_, err = kb.SaveMultisig("multi", multi)
	s.Require().NoError(err)

	s.ac = addresscodec.NewBech32Codec("cosmos")
}

func (s *CLITestSuite) TestCLIValidateSignatures() {
	sendTokens := sdk.NewCoins(
		sdk.NewCoin("testtoken", math.NewInt(10)),
		sdk.NewCoin("stake", math.NewInt(10)))

	res, err := s.createBankMsg(s.clientCtx, s.val, sendTokens,
		fmt.Sprintf("--%s=true", flags.FlagGenerateOnly))
	s.Require().NoError(err)

	// write  unsigned tx to file
	unsignedTx := testutil.WriteToNewTempFile(s.T(), res.String())
	defer unsignedTx.Close()

	res, err = TxSignExec(s.clientCtx, s.val, unsignedTx.Name())
	s.Require().NoError(err)
	signedTx, err := s.clientCtx.TxConfig.TxJSONDecoder()(res.Bytes())
	s.Require().NoError(err)

	signedTxFile := testutil.WriteToNewTempFile(s.T(), res.String())
	defer signedTxFile.Close()
	txBuilder, err := s.clientCtx.TxConfig.WrapTxBuilder(signedTx)
	s.Require().NoError(err)
	_, err = TxValidateSignaturesExec(s.clientCtx, s.val, signedTxFile.Name())
	s.Require().NoError(err)

	txBuilder.SetMemo("MODIFIED TX")
	bz, err := s.clientCtx.TxConfig.TxJSONEncoder()(txBuilder.GetTx())
	s.Require().NoError(err)

	modifiedTxFile := testutil.WriteToNewTempFile(s.T(), string(bz))
	defer modifiedTxFile.Close()

	_, err = TxValidateSignaturesExec(s.clientCtx, s.val, modifiedTxFile.Name())
	s.Require().EqualError(err, "signatures validation failed")
}

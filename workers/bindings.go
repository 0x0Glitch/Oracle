package workers

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type OracleCaller struct {
	contract *bind.BoundContract
}

func NewOracleCaller(address common.Address, client *ethclient.Client) (*OracleCaller, error) {
	parsed, err := OracleMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	contract := bind.NewBoundContract(address, *parsed, client, client, client)
	return &OracleCaller{contract: contract}, nil
}

func (o *OracleCaller) GetUnderlyingPrice(opts *bind.CallOpts, mToken common.Address) (*big.Int, error) {
	var out []interface{}
	err := o.contract.Call(opts, &out, "getUnderlyingPrice", mToken)
	if err != nil {
		return nil, err
	}
	return out[0].(*big.Int), nil
}

var OracleMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"mToken\",\"type\":\"address\"}],\"name\":\"getUnderlyingPrice\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

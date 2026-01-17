package workers

// BaseTokens returns all token configurations for Base chain
func BaseTokens() map[string]TokenMeta {
	return map[string]TokenMeta{
		"aero":   {Symbol: "AERO", MTokAddr: "0x73902f619CEB9B31FD8EFecf435CbDf89E369Ba6", Decimals: 18, TableName: "AERO", PriceAddress: "0x940181a94a35A4569E4529A3cdfB74e38fD98631"},
		"cbbtc":  {Symbol: "cbBTC", MTokAddr: "0xF877ACaFA28c19b96727966690b2f44d35aD5976", Decimals: 8, TableName: "cbBTC", PriceAddress: "0xcbB7C0000aB88B473b1f5aFd9ef808440eed33Bf"},
		"cbeth":  {Symbol: "cbETH", MTokAddr: "0x3bf93770f2d4a794c3d9EBEfBAeBAE2a8f09A5E5", Decimals: 18, TableName: "cbETH", PriceAddress: "0x2Ae3f1EC7F1F5012CfEab0185BfC7Aa3CF0DEc22"},
		"cbxrp":  {Symbol: "cbXRP", MTokAddr: "0xb4fb8fed5b3AaA8434f0B19b1b623d977e07e86d", Decimals: 6, TableName: "cbXRP", PriceAddress: "0xcb585250F852C6C6bf90434AB21A00f02833A4AF"},
		"dai":    {Symbol: "DAI", MTokAddr: "0x73b06D8d18De422E269645eaCe15400DE7462417", Decimals: 18, TableName: "DAI", IsStablecoin: true, PegValue: 1.0, PriceAddress: "0x50c5725949A6F0c72E6C4a641F24049A917DB0Cb"},
		"eurc":   {Symbol: "EURC", MTokAddr: "0xb682c840B5F4FC58B20769E691A6fa1305A501a2", Decimals: 6, TableName: "EURC", IsStablecoin: true, PegValue: 1.16, PriceAddress: "0x60a3e35cC302BfA44Cb288BC5a4F316fdB1Adb42"},
		"lbtc":   {Symbol: "LBTC", MTokAddr: "0x10fF57877b79e9bd949B3815220eC87B9fc5D2ee", Decimals: 8, TableName: "LBTC", PriceAddress: "0xecAc9C5F704e954931349Da37F60E39f515c11c1"},
		"mamo":   {Symbol: "MAMO", MTokAddr: "0x2F90Bb22eB3979f5FfAd31EA6C3F0792ca66dA32", Decimals: 18, TableName: "MAMO", PriceAddress: "0x7300B37DfdfAb110d83290A29DfB31B1740219fE"},
		"morpho": {Symbol: "MORPHO", MTokAddr: "0x6308204872BdB7432dF97b04B42443c714904F3E", Decimals: 18, TableName: "MORPHO", PriceAddress: "0xBAa5CC21fd487B8Fcc2F632f3F4E8D37262a0842"},
		"reth":   {Symbol: "rETH", MTokAddr: "0xcb1dacd30638ae38f2b94ea64f066045b7d45f44", Decimals: 18, TableName: "rETH", PriceAddress: "0xB6fe221Fe9EeF5aBa221c348bA20A1Bf5e73624c"},
		"tbtc":   {Symbol: "tBTC", MTokAddr: "0x9A858ebfF1bEb0D3495BB0e2897c1528eD84A218", Decimals: 18, TableName: "tBTC", PriceAddress: "0x236aa50979d5f3de3bd1eeb40e81137f22ab794b"},
		"usdbc":  {Symbol: "USDbC", MTokAddr: "0x703843C3379b52F9FF486c9f5892218d2a065cC8", Decimals: 6, TableName: "USDbC", IsStablecoin: true, PegValue: 1.0, PriceAddress: "0xd9aAEc86B65D86f6A7B5B1b0c42FFA531710b6CA"},
		"usdc":   {Symbol: "USDC", MTokAddr: "0xEdc817A28E8B93B03976FBd4a3dDBc9f7D176c22", Decimals: 6, TableName: "USDC", IsStablecoin: true, PegValue: 1.0, PriceAddress: "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"},
		"usds":   {Symbol: "USDS", MTokAddr: "0xb6419c6C2e60c4025D6D06eE4F913ce89425a357", Decimals: 18, TableName: "USDS", IsStablecoin: true, PegValue: 1.0, PriceAddress: "0x820C137Fa70C8691F0E44dC420A5E53C168921DC"},
		"weeth":  {Symbol: "weETH", MTokAddr: "0xb8051464C8c92209C92F3a4CD9C73746C4c3CFb3", Decimals: 18, TableName: "weETH", PriceAddress: "0x04c0599Ae5A44757c0AF6F9Ec3B93DA8976c150a"},
		"well":   {Symbol: "WELL", MTokAddr: "0xdC7810B47eAAb250De623F0eE07764afa5F71ED1", Decimals: 18, TableName: "WELL", PriceAddress: "0xA88594D404727625A9437C3f886C7643872296AE"},
		"weth":   {Symbol: "WETH", MTokAddr: "0x628ff693426583D9a7FB391E54366292F509D457", Decimals: 18, TableName: "WETH", PriceAddress: "0x4200000000000000000000000000000000000006"},
		"wrseth": {Symbol: "wrsETH", MTokAddr: "0xfC41B49d064Ac646015b459C522820DB9472F4B5", Decimals: 18, TableName: "wrsETH", PriceAddress: "0xEDfa23602D0EC14714057867A78d01e94176BEA0"},
		"wsteth": {Symbol: "wstETH", MTokAddr: "0x627Fe393Bc6EdDA28e99AE648fD6fF362514304b", Decimals: 18, TableName: "wstETH", PriceAddress: "0xc1CBa3fCea344f92D9239c08C0568f6F2F0ee452"},
	}
}

func OptimismTokens() map[string]TokenMeta {
	return map[string]TokenMeta{
		"dai":     {Symbol: "DAI", MTokAddr: "0x3FE782C2Fe7668C2F1Eb313ACf3022a31feaD6B2", Decimals: 18, TableName: "DAI", IsStablecoin: true, PegValue: 1.0, PriceAddress: "0xDA10009cBd5D07dd0CeCc66161FC93D7c9000da1"},
		"usdc":    {Symbol: "USDC", MTokAddr: "0x8E08617b0d66359D73Aa11E11017834C29155525", Decimals: 6, TableName: "USDC", IsStablecoin: true, PegValue: 1.0, PriceAddress: "0x0b2C639c533813f4Aa9D7837CAf62653d097Ff85"},
		"weth":    {Symbol: "WETH", MTokAddr: "0xb4104C02BBf4E9be85AAa41a62974E4e28D59A33", Decimals: 18, TableName: "WETH", PriceAddress: "0x4200000000000000000000000000000000000006"},
		"cbeth":   {Symbol: "cbETH", MTokAddr: "0x95C84F369bd0251ca903052600A3C96838D78bA1", Decimals: 18, TableName: "cbETH", PriceAddress: "0xadDb6A0412DE1BA0F936DCaeb8Aaa24578dcF3B2"},
		"wsteth":  {Symbol: "wstETH", MTokAddr: "0xbb3b1aB66eFB43B10923b87460c0106643B83f9d", Decimals: 18, TableName: "wstETH", PriceAddress: "0x1F32b1c2345538c0c6f582fCB022739c4A194Ebb"},
		"reth":    {Symbol: "rETH", MTokAddr: "0x4c2E35E3eC4A0C82849637BC04A4609Dbe53d321", Decimals: 18, TableName: "rETH", PriceAddress: "0x9Bcef72be871e61ED4fBbc7630889beE758eb81D"},
		"weeth":   {Symbol: "weETH", MTokAddr: "0xb8051464c8c92209c92f3a4cd9c73746c4c3cfb3", Decimals: 18, TableName: "weETH", PriceAddress: "0x5A7fACB970D094B6C7FF1df0eA68D99E6e73CBFF"},
		"wrseth":  {Symbol: "wrsETH", MTokAddr: "0x181bA797ccF779D8aB339721ED6ee827E758668e", Decimals: 18, TableName: "wrsETH", PriceAddress: "0x87eEE96D50Fb761AD85B1c982d28A042169d61b1"},
		"wbtc":    {Symbol: "WBTC", MTokAddr: "0x6e6CA598A06E609c913551B729a228B023f06fDB", Decimals: 8, TableName: "WBTC", PriceAddress: "0x68f180fcCe6836688e9084f035309E29Bf0A2095"},
		"usdt":    {Symbol: "USDT", MTokAddr: "0xa3A53899EE8f9f6E963437C5B3f805FEc538BF84", Decimals: 6, TableName: "USDT", IsStablecoin: true, PegValue: 1.0, PriceAddress: "0x94b008aA00579c1307B0EF2c499aD98a8ce58e58"},
		"op":      {Symbol: "OP", MTokAddr: "0x9fc345a20541Bf8773988515c5950eD69aF01847", Decimals: 18, TableName: "OP", PriceAddress: "0x4200000000000000000000000000000000000042"},
		"velo":    {Symbol: "VELO", MTokAddr: "0x866b838b97ee43f2c818b3cb5cc77a0dc22003fc", Decimals: 18, TableName: "VELO", PriceAddress: "0x9560e827aF36c94D2Ac33a39bCE1Fe78631088Db"},
		"usdt0":   {Symbol: "USDT0", MTokAddr: "0xed37cD7872c6fe4020982d35104bE7919b8f8b33", Decimals: 6, TableName: "USDT0", IsStablecoin: true, PegValue: 1.0, PriceAddress: "0x01bFF41798a0BcF287b996046Ca68b395DbC1071"},
	}
}

func MoonbeamTokens() map[string]TokenMeta {
	return map[string]TokenMeta{
		"glmr":    {Symbol: "GLMR", MTokAddr: "0x091608f4e4a15335145be0a279483c0f8e4c7955", Decimals: 18, TableName: "GLMR", SkipDEXPrice: true},
		"xcdot":   {Symbol: "xcDOT", MTokAddr: "0xd22da948c0ab3a27f5570b604f3adef5f68211c3", Decimals: 10, TableName: "xcDOT", PriceAddress: "0xFfFFfFff1FcaCBd218EDc0EbA20Fc2308C778080"},
		"frax":    {Symbol: "FRAX", MTokAddr: "0x1C55649f73CDA2f72CEf3DD6C5CA3d49EFcF484C", Decimals: 18, TableName: "FRAX", IsStablecoin: true, PegValue: 1.0, PriceAddress: "0x322E86852e492a7Ee17f28a78c663da38FB33bfb"},
		"xcusdc":  {Symbol: "xcUSDC", MTokAddr: "0x22b1a40e3178fe7c7109efcc247c5bb2b34abe32", Decimals: 6, TableName: "xcUSDC", IsStablecoin: true, PegValue: 1.0, PriceAddress: "0xFFfffffF7D2B0B761Af01Ca8e25242976ac0aD7D"},
		"xcusdt":  {Symbol: "xcUSDT", MTokAddr: "0x42a96c0681b74838ec525adbd13c37f66388f289", Decimals: 6, TableName: "xcUSDT", IsStablecoin: true, PegValue: 1.0, PriceAddress: "0xFFFFFFfFea09FB06d082fd1275CD48b191cbCD1d"},
		"ethwh":   {Symbol: "ETH.wh", MTokAddr: "0xb6c94b3a378537300387b57ab1cc0d2083f9aeac", Decimals: 18, TableName: "ETH_wh", PriceAddress: "0xab3f0245B83feB11d15AAffeFD7AD465a59817eD"},
		"btcwh":   {Symbol: "BTC.wh", MTokAddr: "0xaaa20c5a584a9fecdfedd71e46da7858b774a9ce", Decimals: 8, TableName: "BTC_wh", PriceAddress: "0xE57eBd2d67B462E9926e04a8e33f01cD0D64346D"},
		"usdcwh":  {Symbol: "USDC.wh", MTokAddr: "0x744b1756e7651c6d57f5311767eafe5e931d615b", Decimals: 6, TableName: "USDC_wh", IsStablecoin: true, PegValue: 1.0, PriceAddress: "0x931715FEE2d06333043d11F658C8CE934aC61D0c"},
	}
}

func MoonriverTokens() map[string]TokenMeta {
	return map[string]TokenMeta{
		"movr":  {Symbol: "MOVR", MTokAddr: "0x6a1A771C7826596652daDC9145fEAaE62b1cd07f", Decimals: 18, TableName: "MOVR", SkipDEXPrice: true},
		"xcksm": {Symbol: "xcKSM", MTokAddr: "0xa0d116513bd0b8f3f14e6ea41556c6ec34688e0f", Decimals: 12, TableName: "xcKSM", PriceAddress: "0xFfFFfFff1FcaCBd218EDc0EbA20Fc2308C778080"},
		"frax":  {Symbol: "FRAX", MTokAddr: "0x93Ef8B7c6171BaB1C0A51092B2c9da8dc2ba0e9D", Decimals: 18, TableName: "FRAX", IsStablecoin: true, PegValue: 1.0, PriceAddress: "0x1A93B23281CC1CDE4C4741353F3064709A16197d"},
	}
}

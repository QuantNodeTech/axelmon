package api

type Client struct {
	axelar     string
	axelarscan string
}

type RecordsRequest struct {
	Method   string `json:"method"`
	Chain    string `json:"chain"`
	Size     int    `json:"size"`
	FromTime int64  `json:"fromTime"`
}

type Proxy struct {
	Height string `json:"height"`
	Result struct {
		Address string `json:"address"`
		Status  string `json:"status"`
	} `json:"result"`
}

type VerifierAccount struct {
	Address         string   `json:"address"`
	SupportedChains []string `json:"supportedChains"`
}

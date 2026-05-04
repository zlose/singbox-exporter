package singbox

type ProxiesResponse struct {
	Proxies map[string]Proxy `json:"proxies"`
}

type Proxy struct {
	Name string `json:"-"`
	Type string `json:"type"`
}

type DelayResponse struct {
	Delay int `json:"delay"`
}

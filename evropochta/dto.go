package evropochta

import "encoding/json"

type requestParams struct {
	CRC    string     `json:"CRC"`
	Packet baseParams `json:"Packet"`
}

type baseParams struct {
	JWT           string      `json:"JWT"`
	MethodName    string      `json:"MethodName"`
	ServiceNumber string      `json:"ServiceNumber"`
	Data          interface{} `json:"Data"`
}

type getTokenData struct {
	LoginName       string `json:"LoginName"`
	Password        string `json:"Password"`
	LoginNameTypeId string `json:"LoginNameTypeId"`
}

type getStickerSerialNumber struct {
	SerialNumber string `json:"SerialNumber"`
}

type getStickerData struct {
	SerialNumber []getStickerSerialNumber `json:"SerialNumber"`
}

type errResponseItem struct {
	Error            string `json:"Error"`
	ErrorDescription string `json:"ErrorDescription"`
	ErrorInfo        string `json:"ErrorInfo"`
}

type getTokenResponseItem struct {
	JWT string
}

type genericResponse struct {
	Table []json.RawMessage `json:"Table"`
}

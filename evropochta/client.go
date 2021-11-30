package evropochta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	log "github.com/go-kit/kit/log"
)

type Evropochta interface {
	GetPostSticker(ctx context.Context, trackNum string) (*Sticker, error)
	GetToken(ctx context.Context) error
	HasToken() bool
}

type Sticker struct {
	FileName  string
	FileData  []byte
	LoсalPath string
}

type client struct {
	client        *http.Client
	url           *url.URL
	user          string
	pass          string
	serviceNumber string
	outFolder     string
	jwt           string
	logger        log.Logger
}

func NewClient(baseURL, user, pass, serviceNumber, outFolder string, logger log.Logger) (Evropochta, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	return &client{
		url:           u,
		user:          user,
		pass:          pass,
		serviceNumber: serviceNumber,
		outFolder:     outFolder,
		client:        &http.Client{Timeout: time.Second * 10},
		logger:        logger,
	}, nil
}

func (c *client) HasToken() bool {
	return c.jwt != ""
}

func (c *client) GetPostSticker(ctx context.Context, trackNum string) (*Sticker, error) {
	if c.jwt == "" {
		err := c.GetToken(ctx)
		if err != nil {
			return nil, err
		}
	}

	param := requestParams{
		Packet: baseParams{
			JWT:           c.jwt,
			MethodName:    "Postal.GetPDF",
			ServiceNumber: c.serviceNumber,
			Data: getStickerData{
				SerialNumber: []getStickerSerialNumber{{SerialNumber: trackNum}},
			},
		},
	}
	buffer := new(bytes.Buffer)
	err := json.NewEncoder(buffer).Encode(param)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, "POST", c.url.String(), buffer)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "*/*")
	resp, err := c.client.Do(request)
	if err != nil {
		c.jwt = ""
		return nil, err
	}

	defer resp.Body.Close()

	sticker := &Sticker{
		FileName: fmt.Sprintf("%s.pdf", trackNum),
	}
	sticker.FileData, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		if len(sticker.FileData) == 0 {
			c.jwt = ""
			return nil, fmt.Errorf("got empty response body")
		}
		// Try to parse error
		var rawResp genericResponse
		err = json.Unmarshal(sticker.FileData, &rawResp)
		if err == nil && len(rawResp.Table) != 0 {
			// some error response
			c.jwt = ""
			var errResp errResponseItem
			err = json.Unmarshal(rawResp.Table[0], &errResp)
			if err != nil {
				return nil, fmt.Errorf("error while decoding to `Error response`: %w", err)
			}
			return nil, fmt.Errorf("error while getting sticker: %s, %s, %s", errResp.Error, errResp.ErrorDescription, errResp.ErrorInfo)
		}
	default:
		return nil, fmt.Errorf("error while getting sticker, status: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return sticker, nil
}

func (c *client) GetToken(ctx context.Context) error {
	param := requestParams{
		Packet: baseParams{
			JWT:           "null",
			MethodName:    "GetJWT",
			ServiceNumber: c.serviceNumber,
			Data: getTokenData{
				LoginName:       c.user,
				Password:        c.pass,
				LoginNameTypeId: "1",
			},
		},
	}
	buffer := new(bytes.Buffer)
	err := json.NewEncoder(buffer).Encode(param)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, "POST", c.url.String(), buffer)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "*/*")

	resp, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var rawResp genericResponse
		err = json.Unmarshal(data, &rawResp)
		if err != nil {
			return fmt.Errorf("error while decoding `Table` response: %w", err)
		}
		if len(rawResp.Table) == 0 {
			return fmt.Errorf("empty response, Table is empty")
		}

		var errResp errResponseItem
		err = json.Unmarshal(rawResp.Table[0], &errResp)
		if err != nil {
			return fmt.Errorf("error while decoding to `Error response`: %w", err)
		}
		if errResp.Error != "" {
			return fmt.Errorf("error while getting token: %s, %s, %s", errResp.Error, errResp.ErrorDescription, errResp.ErrorInfo)
		}

		var tokenResp getTokenResponseItem
		err = json.Unmarshal(rawResp.Table[0], &tokenResp)
		if err != nil {
			return fmt.Errorf("error while decoding to `token response`: %w", err)
		}
		if tokenResp.JWT == "" {
			return fmt.Errorf("got empty token, or unknown struct")
		}

		c.jwt = tokenResp.JWT
		return nil
	default:
		return fmt.Errorf("error while getting token status: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
}

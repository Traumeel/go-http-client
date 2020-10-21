package go_http_client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"
)

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

type RequestOption func(*http.Request) error
type ResponseParser func(*http.Response) error
type ValidateResponse func(*http.Response) error
type Option func(*Client)

// WithHttpClient setup a custom http client
func WithHttpClient(client httpClient) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithRequestFilters setup a custom logger
func WithLog(l *log.Logger) Option {
	return func(c *Client) {
		c.log = l
	}
}

// WithRequestFilters add filters which will be applied to all the requests
func WithRequestOptions(fn ...RequestOption) Option {
	return func(c *Client) {
		c.requestOptionsChain = append(c.requestOptionsChain, fn...)
	}
}

// RequestBasicAuthOption add basic credentials to all the requests
func RequestBasicAuthOption(username, password string) Option {
	return func(c *Client) {
		c.requestOptionsChain = append(c.requestOptionsChain, func(req *http.Request) (e error) {
			req.SetBasicAuth(username, password)
			return
		})
	}
}

// WithDebug enable debugging for the client
func WithDebug(b bool) Option {
	return func(c *Client) {
		c.debug = b
	}
}

// WithResponseValidator set a custom response validator function
func WithResponseValidator(v ValidateResponse) Option {
	return func(c *Client) {
		c.validateResponseFn = v
	}
}

type Client struct {
	endpoint            string
	log                 *log.Logger
	httpClient          httpClient
	requestOptionsChain []RequestOption
	validateResponseFn  ValidateResponse
	debug               bool
}

func NewClient(endpoint string, options ...Option) *Client {
	c := &Client{
		endpoint:            endpoint,
		httpClient:          &http.Client{Timeout: 30 * time.Second},
		log:                 log.New(),
		requestOptionsChain: make([]RequestOption, 0),
		validateResponseFn:  ResponseValidator,
		debug:               false,
	}

	for _, opt := range options {
		opt(c)
	}

	return c
}

// WithQueryOpt add query to request
func WithQueryOpt(query url.Values) RequestOption {
	return func(req *http.Request) (e error) {
		if query == nil || req == nil {
			return fmt.Errorf("WithQueryOpt error: %v | %v", req, query)
		}
		req.URL.RawQuery = query.Encode()
		return
	}
}

// WithHeadersOpt add headers to a request
func WithHeadersOpt(header http.Header) RequestOption {
	return func(req *http.Request) (e error) {
		if header == nil || req == nil {
			return fmt.Errorf("WithHeadersOpt error: %v | %v", req, header)
		}
		req.Header = header
		return
	}
}

// RequestBodyOption add body to a request
func WithBodyOpt(body io.Reader) RequestOption {
	return func(req *http.Request) (e error) {
		if body == nil || req == nil {
			return fmt.Errorf("WithBodyOpt error: %v | %v", req, body)
		}
		nreq, err := http.NewRequest("", req.URL.String(), body)
		if err != nil {
			return err
		}

		req.Body = nreq.Body
		req.GetBody = nreq.GetBody
		req.ContentLength = nreq.ContentLength
		return
	}
}

func RawStringParser(dst *string) ResponseParser {
	return func(resp *http.Response) (e error) {
		if resp == nil || dst == nil {
			return fmt.Errorf("RawStringParser function error: %v | %v", resp, dst)
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read resp body: %w", err)
		}
		*dst = string(body)
		return
	}
}

func RawBodyParser(dst *[]byte) ResponseParser {
	return func(resp *http.Response) (e error) {
		if resp == nil || dst == nil {
			return fmt.Errorf("RawBodyParser function error: %v | %v", resp, dst)
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read resp body: %w", err)
		}
		*dst = body
		return
	}
}

func NoBodyParser(log *log.Logger) ResponseParser {
	return func(resp *http.Response) (e error) {
		if resp.ContentLength != 0 && log != nil {
			logResponse(resp, log)
		}
		return
	}
}

func JsonParser(dst interface{}) ResponseParser {
	return func(resp *http.Response) (e error) {
		if resp == nil || dst == nil {
			return fmt.Errorf("JsonParser function error: %v | %v", resp, dst)
		}
		return json.NewDecoder(resp.Body).Decode(dst)
	}
}

func ResponseValidator(resp *http.Response) error {
	if resp.StatusCode > 300 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		return StatusCodeError{
			Code:   resp.StatusCode,
			Status: resp.Status,
			Body:   string(body),
		}
	}

	return nil
}

func logRequest(req *http.Request, log *log.Logger) {
	requestDump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		log.WithError(err).Error("failed to dump http request for logging")
		return
	}
	log.Infof(string(requestDump))
}

func logResponse(resp *http.Response, log *log.Logger) {
	respDump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		log.WithError(err).Error("failed to dump http response for logging")
		return
	}
	log.Infof(string(respDump))
}

// StatusCodeError represents an http response error
type StatusCodeError struct {
	Code   int
	Status string
	Body   string
}

func (t StatusCodeError) Error() string {
	return fmt.Sprintf("error: %v | %v | %v", t.Code, t.Status, t.Body)
}

func (t StatusCodeError) HTTPStatusCode() int {
	return t.Code
}

func (c *Client) GetJson(ctx context.Context, path string, intf interface{}, options ...RequestOption) error {
	return c.DoRequestJson(ctx, http.MethodGet, path, intf, options...)
}

func (c *Client) DoRequestJson(ctx context.Context, method, path string, intf interface{}, options ...RequestOption) error {
	return c.DoRequest(ctx, method, path, JsonParser(intf), options...)
}

func (c *Client) Get(ctx context.Context, path string, options ...RequestOption) error {
	return c.DoRequestNoBody(ctx, http.MethodGet, path, options...)
}

func (c *Client) DoRequestNoBody(ctx context.Context, method, path string, options ...RequestOption) error {
	return c.DoRequest(ctx, method, path, NoBodyParser(c.log), options...)
}

func (c *Client) DoRequestString(ctx context.Context, method, path string, out *string, options ...RequestOption) error {
	return c.DoRequest(ctx, method, path, RawStringParser(out), options...)
}

func (c *Client) DoRequest(ctx context.Context, method, path string, parser ResponseParser, options ...RequestOption) error {
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, nil)
	if err != nil {
		return err
	}

	//apply global request options
	for _, opt := range c.requestOptionsChain {
		if err := opt(req); err != nil {
			return fmt.Errorf("failed to apply global request option: %w", err)
		}
	}

	//apply custom request options
	for _, opt := range options {
		if err := opt(req); err != nil {
			return fmt.Errorf("failed to apply global request option: %w", err)
		}
	}

	if c.debug {
		logRequest(req, c.log)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if c.debug {
		logResponse(resp, c.log)
	}

	if err := c.validateResponseFn(resp); err != nil {
		return err
	}

	return parser(resp)
}

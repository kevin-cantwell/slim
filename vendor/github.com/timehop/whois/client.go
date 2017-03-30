package whois

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/timehop/services/request"
	"golang.org/x/net/context"
)

var (
	ErrClientTimeout      = WhoisError{Desc: "server did not respond within timeout", StatusCode: http.StatusRequestTimeout, Transient: true}
	ErrMalformedAuthToken = WhoisError{Desc: "malformed auth token ", StatusCode: http.StatusBadRequest, Transient: false}
	ErrNotFound           = WhoisError{Desc: "resource does not exist", StatusCode: http.StatusNotFound, Transient: false}
)

type Client struct {
	options options
}

type options struct {
	url           string
	clientTimeout time.Duration
	httpClient    *http.Client
}

type Option func(*options)

// If left unset, the client will use https://timehop.com
func WithServerURL(url string) Option {
	return func(o *options) {
		o.url = url
	}
}

// WithClientTimeout sets the amount of time the client will wait for the server to respond
// before returning with an ErrClientTimeout error. It is not equivalent to the *http.Client
// timeout, which can be set with the WithHTTPClient option.
//
// If left unset, a default timeout of 2 seconds will be used. Explicitly setting the
// timeout to less than or equal to zero will result in no timeout being set.
func WithClientTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.clientTimeout = timeout
	}
}

// WithHTTPClient sets the http.Client to use for all http calls.
//
// If left unset or nil, a default client will be constructed
// with a KeepAlive of 30 seconds and a timeout of 2 seconds.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(o *options) {
		if httpClient != nil {
			o.httpClient = httpClient
		}
	}
}

func NewClient(opts ...Option) Client {
	defaultTimeout := 2 * time.Second
	client := Client{
		options: options{
			url:           "https://timehop.com",
			clientTimeout: defaultTimeout,
			httpClient: &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyFromEnvironment,
					Dial: (&net.Dialer{
						Timeout:   defaultTimeout,
						KeepAlive: 30 * time.Second,
					}).Dial,
					TLSHandshakeTimeout: defaultTimeout,
				},
				Timeout: defaultTimeout,
			},
		},
	}

	for _, opt := range opts {
		opt(&client.options)
	}

	return client
}

/*
  Accounts
*/

func (c Client) AccountByID(accountID int64) (*Account, error) {
	url := fmt.Sprintf("%s/accounts/%d", c.options.url, accountID)
	req, _ := http.NewRequest("GET", url, nil)
	return c.fetchAccount(req)
}

func (c Client) FacebookAccountByUID(uid string) (*Account, error) {
	return c.accountByTypeAndUID(FacebookTypeKey, uid)
}

func (c Client) InstagramAccountByUID(uid string) (*Account, error) {
	return c.accountByTypeAndUID(InstagramTypeKey, uid)
}

func (c Client) TwitterAccountByUID(uid string) (*Account, error) {
	return c.accountByTypeAndUID(TwitterTypeKey, uid)
}

func (c Client) FoursquareAccountByUID(uid string) (*Account, error) {
	return c.accountByTypeAndUID(FoursquareTypeKey, uid)
}

func (c Client) DropboxAccountByUID(uid string) (*Account, error) {
	return c.accountByTypeAndUID(DropboxTypeKey, uid)
}

func (c Client) GoogleAccountByUID(uid string) (*Account, error) {
	return c.accountByTypeAndUID(GoogleTypeKey, uid)
}

func (c Client) DeauthorizeAccountByID(accountID int64) error {
	url := fmt.Sprintf("%s/accounts/%d/authorization", c.options.url, accountID)
	req, _ := http.NewRequest("DELETE", url, nil)

	resp, err := c.DoRequest(req)
	switch err.(type) {
	case nil:
		// Do nothing
	case WhoisError:
		return err
	default:
		// We are purposefully making these unknown errors as transient, per discussion
		// in this thread: https://github.com/timehop/whois/pull/93#discussion_r37760551
		// TL;DR Setting it to false is explicitly forbidding upstream users of this
		// function from retrying. We are letting the caller determine how to handle
		// errors.
		return NewError(err.Error(), 0, true)
	}

	if resp.StatusCode != http.StatusNoContent {
		desc := fmt.Sprintf("whois/client: unexpected status code during deauthorization. status: %d", resp.StatusCode)
		return NewError(desc, resp.StatusCode, isHTTPStatusCodeTransient(resp.StatusCode))
	}

	return nil
}

func (c Client) AddAuthToken(userID int64, authToken string) error {
	urlStr := fmt.Sprintf("%s/users/%d/app_auth_token", c.options.url, userID)
	values := make(url.Values)
	values.Set("auth_token", authToken)
	req, _ := http.NewRequest("PUT", urlStr, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; v=2")
	err := c.FetchModel(req, nil)
	switch err.(type) {
	case nil:
		// Do nothing
	case WhoisError:
		return err
	default:
		// We are purposefully making these unknown errors as transient, per discussion
		// in this thread: https://github.com/timehop/whois/pull/93#discussion_r37760551
		// TL;DR Setting it to false is explicitly forbidding upstream users of this
		// function from retrying. We are letting the caller determine how to handle
		// errors.
		return NewError(err.Error(), 0, true)
	}

	return nil
}

func (c Client) DeleteAccountByID(accountID int64) error {
	url := fmt.Sprintf("%s/accounts/%d", c.options.url, accountID)
	req, _ := http.NewRequest("DELETE", url, nil)

	resp, err := c.DoRequest(req)
	switch err.(type) {
	case nil:
		// Do nothing
	case WhoisError:
		return err
	default:
		// We are purposefully making these unknown errors as transient, per discussion
		// in this thread: https://github.com/timehop/whois/pull/93#discussion_r37760551
		// TL;DR Setting it to false is explicitly forbidding upstream users of this
		// function from retrying. We are letting the caller determine how to handle
		// errors.
		return NewError(err.Error(), 0, true)
	}

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}

	if resp.StatusCode != http.StatusNoContent {
		desc := fmt.Sprintf("whois/client: unexpected status code during account deletion. status: %d", resp.StatusCode)
		return NewError(desc, resp.StatusCode, isHTTPStatusCodeTransient(resp.StatusCode))
	}

	return nil
}

/*
  Users
*/

func (c Client) UserByID(userID int64, fields ...UserField) (*User, error) {
	strFields := make([]string, len(fields))
	for i, field := range fields {
		strFields[i] = string(field)
	}
	query := "fields=" + strings.Join(strFields, ",")
	url := fmt.Sprintf("%s/users/%d?%s", c.options.url, userID, query)

	req, _ := http.NewRequest("GET", url, nil)
	return c.fetchUser(req)
}

func (c Client) UserByAuthToken(authToken string, fields ...UserField) (*User, error) {
	if !isValidAuthToken(authToken) {
		return nil, ErrMalformedAuthToken
	}

	strFields := make([]string, len(fields))
	for i, field := range fields {
		strFields[i] = string(field)
	}
	query := "auth_token=" + authToken + "&fields=" + strings.Join(strFields, ",")
	url := fmt.Sprintf("%s/users/me?%s", c.options.url, query)

	req, _ := http.NewRequest("GET", url, nil)
	return c.fetchUser(req)
}

func (c Client) UpdateTwitterInfo(accountID int64, info TwitterInfo) error {
	values := make(url.Values)

	values.Set("screen_name", info.ScreenName)
	values.Set("statuses_count", info.StatusesCount.String())
	values.Set("name", info.Name)
	values.Set("id_str", info.IDStr)
	values.Set("profile_image_url", info.ProfileImageURL)
	values.Set("time_zone", info.TimeZone)
	values.Set("verified", fmt.Sprint(info.Verified))

	u := fmt.Sprintf("%v/accounts/%v/twitter_info", c.options.url, accountID)
	req, err := http.NewRequest("PATCH", u, strings.NewReader(values.Encode()))
	if err != nil {
		msg := fmt.Sprintf("whois/client: could not construct request: %v", err)
		return NewError(msg, 0, false)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; v=2")

	response, err := c.DoRequest(req)
	if err != nil {
		return NewError(err.Error(), 0, true)
	}
	defer response.Body.Close()

	switch {
	case response.StatusCode == 404:
		return ErrNotFound

	case response.StatusCode > 299:
		body, _ := ioutil.ReadAll(response.Body)
		msg := fmt.Sprintf("whois/client: %v error received from whois/server: %v", response.StatusCode, string(body))
		return NewError(msg, response.StatusCode, isHTTPStatusCodeTransient(response.StatusCode))
	}

	return nil
}

func (c Client) UpdateUser(userID int64, attrs map[string]interface{}) error {
	values := make(url.Values)

	for k, v := range attrs {
		values.Set(k, fmt.Sprint(v))
	}

	url := fmt.Sprintf("%v/users/%v", c.options.url, userID)
	req, err := http.NewRequest("PATCH", url, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; v=2")
	if err != nil {
		msg := fmt.Sprintf("whois/client: could not construct request: %v", err)
		return NewError(msg, 0, false)
	}

	response, err := c.DoRequest(req)
	if err != nil {
		return NewError(err.Error(), 0, true)
	}

	defer response.Body.Close()

	switch {
	case response.StatusCode == 404:
		return ErrNotFound

	case response.StatusCode > 299:
		body, _ := ioutil.ReadAll(response.Body)
		msg := fmt.Sprintf("whois/client: %v error received from whois/server: %v", response.StatusCode, string(body))
		return NewError(msg, response.StatusCode, isHTTPStatusCodeTransient(response.StatusCode))
	}

	return nil
}

func (c Client) UpdateUserPreferences(userID int64, attrs map[string]interface{}) error {
	values := make(url.Values)

	for k, v := range attrs {
		values.Set(k, fmt.Sprint(v))
	}

	url := fmt.Sprintf("%v/users/%v/preferences", c.options.url, userID)
	req, err := http.NewRequest("PATCH", url, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; v=2")
	if err != nil {
		msg := fmt.Sprintf("whois/client: could not construct request: %v", err)
		return NewError(msg, 0, false)
	}

	response, err := c.DoRequest(req)
	if err != nil {
		return NewError(err.Error(), 0, true)
	}
	defer response.Body.Close()

	switch {
	case response.StatusCode == http.StatusNotFound:
		return ErrNotFound

	case response.StatusCode > 299:
		body, _ := ioutil.ReadAll(response.Body)
		msg := fmt.Sprintf("whois/client: %v error received from whois/server: %v", response.StatusCode, string(body))
		return NewError(msg, response.StatusCode, isHTTPStatusCodeTransient(response.StatusCode))
	}

	return nil
}

// TODO we will want to fold this into the other one or even more into the core client object.
func (c Client) UserByAuthTokenWithContext(ctx context.Context, authToken string, fields ...UserField) (*User, error) {
	if !isValidAuthToken(authToken) {
		return nil, ErrMalformedAuthToken
	}

	strFields := make([]string, len(fields))
	for i, field := range fields {
		strFields[i] = string(field)
	}
	query := "auth_token=" + authToken + "&fields=" + strings.Join(strFields, ",")
	url := fmt.Sprintf("%s/users/me?%s", c.options.url, query)
	req, _ := http.NewRequest("GET", url, nil)

	request.SetHTTPReqRIDHeader(ctx, req)

	return c.fetchUser(req)
}

func (c Client) fetchUser(req *http.Request) (*User, error) {
	var userWithFields User
	if err := c.FetchModel(req, &userWithFields); err != nil {
		return nil, err
	}
	// About 25k users had their birthdays accidentally set to 00010101. This helps us recover from that
	if userWithFields.Birthdate != nil && (userWithFields.Birthdate.Year == 1 || userWithFields.Birthdate.Year == 2001) {
		userWithFields.Birthdate = nil
	}
	return &userWithFields, nil
}

func (c Client) accountByTypeAndUID(aType string, uid string) (*Account, error) {
	url := fmt.Sprintf("%s/accounts/%s/%s", c.options.url, aType, uid)
	req, _ := http.NewRequest("GET", url, nil)
	return c.fetchAccount(req)
}

func (c Client) fetchAccount(req *http.Request) (*Account, error) {
	var account Account
	if err := c.FetchModel(req, &account); err != nil {
		return nil, err
	}
	return &account, nil
}

// Guarantees we set the right headers for the new stuff
func (c Client) DoRequest(req *http.Request) (*http.Response, error) {
	result := make(chan struct {
		resp *http.Response
		err  error
	})
	go func(req *http.Request) {
		if req.Header.Get("Content-Type") == "" {
			// This header differentiates legacy whois requests from new ones (just in case the resources is identical)
			req.Header.Set("Content-Type", "application/json; v=2")
		}
		resp, err := c.options.httpClient.Do(req)
		result <- struct {
			resp *http.Response
			err  error
		}{resp, err}
	}(req)
	select {
	case <-time.After(c.options.clientTimeout):
		return nil, ErrClientTimeout
	case res := <-result:
		return res.resp, res.err
	}
}

func (c Client) FetchModel(req *http.Request, model interface{}) error {
	resp, err := c.DoRequest(req)
	switch err.(type) {
	case nil:
		// Do nothing
	case WhoisError:
		return err
	default:
		// We are purposefully making these unknown errors as transient, per discussion
		// in this thread: https://github.com/timehop/whois/pull/93#discussion_r37760551
		// TL;DR Setting it to false is explicitly forbidding upstream users of this
		// function from retrying. We are letting the caller determine how to handle
		// errors.
		return NewError(err.Error(), 0, true)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return NewError(err.Error(), resp.StatusCode, false)
	}

	if resp.StatusCode > 299 {
		return c.parseError(body, resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNoContent {
		// Then don't unmarshal
		return nil
	}
	if err := json.Unmarshal(body, &model); err != nil {
		return NewError(err.Error(), 0, false)
	}

	return nil
}

// Guaranteed to return a non-nil error
func (c Client) parseError(body []byte, statusCode int) error {
	if statusCode == 404 {
		return ErrNotFound
	}

	var whoisErr WhoisError
	err := json.Unmarshal(body, &whoisErr)
	if err != nil {
		desc := fmt.Sprintf("whois/client: could not parse error response from server as JSON. parsing error: %s ; response body: %s", err, body)
		return NewError(desc, statusCode, isHTTPStatusCodeTransient(statusCode))
	}

	return whoisErr
}

func isHTTPStatusCodeTransient(statusCode int) bool {
	switch statusCode {
	case http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

var authTokenPattern = regexp.MustCompile("^[A-Z0-9]{32}$")

func isValidAuthToken(authToken string) bool {
	if authToken == "" {
		return false
	}
	if !authTokenPattern.MatchString(authToken) {
		return false
	}
	return true
}

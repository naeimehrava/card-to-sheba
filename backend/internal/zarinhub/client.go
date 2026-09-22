package zarinhub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/behnam/card-to-sheba/backend/internal/inquiry"
)

const maxResponseBytes = 1 << 20

const (
	messageUpstreamAuth        = "سرویس استعلام در حال حاضر در دسترس نیست. لطفاً بعداً تلاش کنید."
	messageUpstreamUnavailable = "سرویس استعلام موقتاً در دسترس نیست. لطفاً چند لحظه بعد دوباره تلاش کنید."
	messageUpstreamTimeout     = "پاسخی از سرویس استعلام دریافت نشد. لطفاً دوباره تلاش کنید."
	messageUpstreamRejected    = "انجام استعلام برای این شماره کارت ممکن نبود."
	messageNotFound            = "شبای متناظر با این شماره کارت یافت نشد."
	messageCardRejected        = "شماره کارت وارد شده مورد پذیرش سرویس استعلام قرار نگرفت."
)

type Client struct {
	baseURL string
	http    *http.Client
	timeout time.Duration
	logger  *slog.Logger
	auth    *tokenSource
}

type Config struct {
	BaseURL     string
	Username    string
	Password    string
	StaticToken string
	Timeout     time.Duration
	TokenSkew   time.Duration
}

func New(cfg Config, logger *slog.Logger) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.TokenSkew <= 0 {
		cfg.TokenSkew = time.Minute
	}

	c := &Client{
		baseURL: cfg.BaseURL,
		timeout: cfg.Timeout,
		logger:  logger,
		http: &http.Client{

			Timeout: cfg.Timeout + 5*time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        50,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
	}
	c.auth = &tokenSource{
		client:   c,
		username: cfg.Username,
		password: cfg.Password,
		static:   cfg.StaticToken,
		skew:     cfg.TokenSkew,
	}
	return c
}

func (c *Client) CardToIBAN(ctx context.Context, card string) (inquiry.Outcome, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	token, err := c.auth.Token(ctx)
	if err != nil {
		return inquiry.Outcome{}, err
	}

	data, info, err := doJSON[cardToIBANData](ctx, c, "/v5/KYC/CardToIban", cardToIBANRequest{Card: card}, token)
	if isUnauthorized(err) {

		c.auth.Invalidate()
		if token, err = c.auth.Token(ctx); err != nil {
			return outcomeOf(info, nil), err
		}
		data, info, err = doJSON[cardToIBANData](ctx, c, "/v5/KYC/CardToIban", cardToIBANRequest{Card: card}, token)
	}
	if err != nil {
		return outcomeOf(info, nil), err
	}

	return outcomeOf(info, &inquiry.BankDetails{
		IBAN:               data.IBAN,
		BankName:           data.BankName,
		EnglishBankName:    data.EnglishBankName,
		Deposit:            data.Deposit,
		DepositOwners:      data.DepositOwners,
		DepositStatus:      data.DepositStatus,
		DepositDescription: data.DepositDescription,
	}), nil
}

type callInfo struct {
	httpStatus int
	trackID    string
	code       *int
	body       []byte
}

func outcomeOf(info callInfo, data *inquiry.BankDetails) inquiry.Outcome {
	return inquiry.Outcome{
		HTTPStatus:   info.httpStatus,
		TrackID:      info.trackID,
		ProviderCode: info.code,
		RawResponse:  info.body,
		Data:         data,
	}
}

func doJSON[T any](ctx context.Context, c *Client, path string, payload any, token string) (T, callInfo, error) {
	var zero T
	var info callInfo

	body, err := json.Marshal(payload)
	if err != nil {
		return zero, info, inquiry.NewError(inquiry.KindInternal, messageInternal, fmt.Errorf("marshal request: %w", err))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return zero, info, inquiry.NewError(inquiry.KindInternal, messageInternal, fmt.Errorf("build request: %w", err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return zero, info, transportError(ctx, path, err)
	}
	defer resp.Body.Close()

	info.httpStatus = resp.StatusCode
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return zero, info, transportError(ctx, path, fmt.Errorf("read response: %w", err))
	}
	info.body = raw

	var env envelope[T]
	if err := json.Unmarshal(raw, &env); err != nil {

		return zero, info, inquiry.NewError(kindForHTTPStatus(resp.StatusCode), messageForHTTPStatus(resp.StatusCode),
			fmt.Errorf("%s: decode response (http %d): %w", path, resp.StatusCode, err))
	}

	code := env.Meta.Code
	info.code = &code
	info.trackID = env.Meta.TrackID

	if !env.Meta.IsSuccess || code != codeSuccess {
		return zero, info, providerError(path, resp.StatusCode, env.Meta)
	}
	if resp.StatusCode != http.StatusOK {
		return zero, info, inquiry.NewError(kindForHTTPStatus(resp.StatusCode), messageForHTTPStatus(resp.StatusCode),
			fmt.Errorf("%s: unexpected http status %d on a success envelope", path, resp.StatusCode))
	}
	return env.Data, info, nil
}

const messageInternal = "خطای داخلی سامانه رخ داده است."

func transportError(ctx context.Context, path string, err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded) || isTimeout(err):
		return inquiry.NewError(inquiry.KindUpstreamTimeout, messageUpstreamTimeout,
			fmt.Errorf("%s: %w", path, err))
	case errors.Is(ctx.Err(), context.Canceled):
		return inquiry.NewError(inquiry.KindUpstreamUnavailable, messageUpstreamUnavailable,
			fmt.Errorf("%s: request cancelled: %w", path, err))
	default:
		return inquiry.NewError(inquiry.KindUpstreamUnavailable, messageUpstreamUnavailable,
			fmt.Errorf("%s: %w", path, err))
	}
}

func isTimeout(err error) bool {
	var timeoutErr interface{ Timeout() bool }
	return errors.As(err, &timeoutErr) && timeoutErr.Timeout()
}

func providerError(path string, httpStatus int, m meta) error {
	cause := fmt.Errorf("%s: zarinhub code=%d type=%s http=%d trackId=%s message=%q",
		path, m.Code, m.ErrorType, httpStatus, m.TrackID, m.ErrorMessage)

	switch m.Code {
	case codeBadRequest:
		return inquiry.NewError(inquiry.KindValidation, messageCardRejected, cause)
	case codeNotFound, codeListEmpty:
		return inquiry.NewError(inquiry.KindNotFound, messageNotFound, cause)
	case codeUnauthorized, codeInaccessible:
		return inquiry.NewError(inquiry.KindUpstreamAuth, messageUpstreamAuth, cause)
	case codeUnavailable, codeServerError:
		return inquiry.NewError(inquiry.KindUpstreamUnavailable, messageUpstreamUnavailable, cause)
	case codeRequestTimout:
		return inquiry.NewError(inquiry.KindUpstreamTimeout, messageUpstreamTimeout, cause)
	case codeLogicError, codeProviderError:
		return inquiry.NewError(inquiry.KindUpstreamRejected, messageUpstreamRejected, cause)
	default:
		return inquiry.NewError(kindForHTTPStatus(httpStatus), messageForHTTPStatus(httpStatus), cause)
	}
}

func kindForHTTPStatus(status int) inquiry.Kind {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return inquiry.KindUpstreamAuth
	case status == http.StatusNotFound:
		return inquiry.KindNotFound
	case status == http.StatusRequestTimeout || status == http.StatusGatewayTimeout:
		return inquiry.KindUpstreamTimeout
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		return inquiry.KindUpstreamRejected
	case status >= 500:
		return inquiry.KindUpstreamUnavailable
	default:
		return inquiry.KindUpstreamRejected
	}
}

func messageForHTTPStatus(status int) string {
	switch kindForHTTPStatus(status) {
	case inquiry.KindUpstreamAuth:
		return messageUpstreamAuth
	case inquiry.KindNotFound:
		return messageNotFound
	case inquiry.KindUpstreamTimeout:
		return messageUpstreamTimeout
	case inquiry.KindUpstreamUnavailable:
		return messageUpstreamUnavailable
	default:
		return messageUpstreamRejected
	}
}

func isUnauthorized(err error) bool {
	var svcErr *inquiry.Error
	return errors.As(err, &svcErr) && svcErr.Kind == inquiry.KindUpstreamAuth
}

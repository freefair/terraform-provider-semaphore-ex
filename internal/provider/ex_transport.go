package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
)

const exMaximumResponseSize = 16 << 20

type exRequestOptions struct {
	PathParams map[string]string
	Query      map[string]string
	Headers    map[string]string
}

// exAPIError is a sanitized non-success response from the Semaphore EX API.
// It deliberately retains no response body, resolved path parameters, or URL.
type exAPIError struct {
	StatusCode int
	method     string
	route      string
}

// exTransportError is a safe error produced by this helper's response reader.
// Unlike errors returned by the underlying HTTP transport, it contains no URL
// or server-provided content.
type exTransportError struct {
	message string
}

func (e *exTransportError) Error() string {
	return e.message
}

func (e *exAPIError) Error() string {
	return fmt.Sprintf("Semaphore EX API request %s %s returned status %d", e.method, e.route, e.StatusCode)
}

func exNotFound(err error) bool {
	var apiErr *exAPIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 404
}

// exRequest submits a handwritten EX endpoint through the generated client's
// configured transport, preserving its authentication, base path, TLS, and
// request context behavior.
func exRequest(
	ctx context.Context,
	client *apiclient.SemaphoreUI,
	method string,
	route string,
	pathParams map[string]string,
	body any,
	result any,
) error {
	return exRequestWithOptions(ctx, client, method, route, exRequestOptions{PathParams: pathParams}, body, result)
}

func exRequestWithOptions(
	ctx context.Context,
	client *apiclient.SemaphoreUI,
	method string,
	route string,
	options exRequestOptions,
	body any,
	result any,
) error {
	for name := range options.Headers {
		if exTransportIdentityHeader(name) {
			return &exTransportError{message: "Semaphore EX API request headers cannot override transport identity"}
		}
	}

	operation := &runtime.ClientOperation{
		ID:                 "EX" + method + route,
		Method:             method,
		PathPattern:        route,
		ProducesMediaTypes: []string{runtime.JSONMime},
		ConsumesMediaTypes: []string{runtime.JSONMime},
		Params: runtime.ClientRequestWriterFunc(func(request runtime.ClientRequest, _ strfmt.Registry) error {
			for name, value := range options.PathParams {
				if err := request.SetPathParam(name, value); err != nil {
					return err
				}
			}
			for name, value := range options.Query {
				if err := request.SetQueryParam(name, value); err != nil {
					return err
				}
			}
			for name, value := range options.Headers {
				if err := request.SetHeaderParam(name, value); err != nil {
					return err
				}
			}
			if body != nil {
				return request.SetBodyParam(body)
			}
			return nil
		}),
		Reader: runtime.ClientResponseReaderFunc(func(response runtime.ClientResponse, _ runtime.Consumer) (any, error) {
			if response.Code()/100 != 2 {
				return nil, &exAPIError{StatusCode: response.Code(), method: method, route: route}
			}

			if result == nil || response.Code() == 204 {
				return nil, nil
			}

			payload, err := io.ReadAll(io.LimitReader(response.Body(), exMaximumResponseSize+1))
			if err != nil {
				return nil, &exTransportError{message: fmt.Sprintf("could not read successful Semaphore EX API response for %s %s", method, route)}
			}
			if len(payload) > exMaximumResponseSize {
				return nil, &exTransportError{message: fmt.Sprintf("Semaphore EX API response for %s %s exceeds %d bytes", method, route, exMaximumResponseSize)}
			}
			if len(payload) == 0 {
				return nil, nil
			}

			decoder := json.NewDecoder(bytes.NewReader(payload))
			decoder.UseNumber()
			if err := decoder.Decode(result); err != nil {
				return nil, &exTransportError{message: fmt.Sprintf("could not decode successful Semaphore EX API response for %s %s", method, route)}
			}
			return result, nil
		}),
	}

	_, err := client.Transport.SubmitContext(ctx, operation)
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	var apiErr *exAPIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	var transportErr *exTransportError
	if errors.As(err, &transportErr) {
		return transportErr
	}
	return fmt.Errorf("semaphore EX API request %s %s failed", method, route)
}

func exTransportIdentityHeader(name string) bool {
	return strings.EqualFold(name, "Authorization") || strings.EqualFold(name, "Cookie") || strings.EqualFold(name, "Host")
}

package provider

import (
	"errors"
	"github.com/go-openapi/runtime"
	"net/http"
)

// resourceNotFound recognizes both native EX and generated-client responses.
// Only an actual 404 permits forgetting state; authorization and transport
// failures must leave the previous state intact.
func resourceNotFound(err error) bool {
	if exNotFound(err) {
		return true
	}
	var apiError *runtime.APIError
	if errors.As(err, &apiError) {
		return apiError.Code == http.StatusNotFound
	}
	var response interface{ Code() int }
	return errors.As(err, &response) && response.Code() == http.StatusNotFound
}

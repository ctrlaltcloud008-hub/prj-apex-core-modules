package apperror

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/spanner"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ErrorType string

const (
	Transient ErrorType = "TRANSIENT"
	Permanent ErrorType = "PERMANENT"
	Ambiguous ErrorType = "AMBIGUOUS"
)

func Classify(err error) ErrorType {
	if err == nil {
		return Permanent
	}

	if spanner.ErrCode(err) == codes.Aborted {
		return Transient
	}

	if s, ok := status.FromError(err); ok {
		switch s.Code() {
		case codes.Unavailable, codes.ResourceExhausted, codes.DeadlineExceeded:
			return Transient
		case codes.InvalidArgument, codes.NotFound, codes.PermissionDenied, codes.AlreadyExists, codes.FailedPrecondition, codes.Unimplemented:
			return Permanent
		}
	}

	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout:
			return Transient
		case http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound, http.StatusConflict:
			return Permanent
		}
	}

	return Ambiguous
}

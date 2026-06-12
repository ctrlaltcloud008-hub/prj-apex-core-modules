package apperror

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ErrorType
	}{
		{"nil", nil, Permanent},
		{"context canceled (spot preemption)", context.Canceled, Transient},
		{"context deadline exceeded", context.DeadlineExceeded, Transient},
		{"wrapped context canceled", fmt.Errorf("encode: %w", context.Canceled), Transient},
		{"grpc unavailable", status.Error(codes.Unavailable, "x"), Transient},
		{"grpc invalid argument", status.Error(codes.InvalidArgument, "x"), Permanent},
		{"grpc not found", status.Error(codes.NotFound, "x"), Permanent},
		{"generic error", errors.New("something broke"), Ambiguous},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.err); got != tt.want {
				t.Errorf("Classify(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

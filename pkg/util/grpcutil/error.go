package grpcutil

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func Error(code codes.Code, err error) error {
	return status.Error(code, err.Error())
}

func ErrorNotFound(err error) error {
	return Error(codes.NotFound, err)
}

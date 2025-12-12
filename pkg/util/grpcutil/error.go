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

func ErrorInternal(err error) error {
	return Error(codes.Internal, err)
}

func ErrorInvalid(err error) error {
	return Error(codes.InvalidArgument, err)
}

func IsError(code codes.Code, err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return st.Code() == code
}

func IsErrorNotFound(err error) bool {
	return IsError(codes.NotFound, err)
}

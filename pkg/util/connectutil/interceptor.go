package connectutil

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/grpc/status"
)

// StatusInterceptor maps gRPC status errors returned by handlers onto their
// connect equivalents; connect-go passes them through as CodeUnknown.
func StatusInterceptor() connect.Interceptor {
	return statusInterceptor{}
}

type statusInterceptor struct{}

func (statusInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		resp, err := next(ctx, req)
		return resp, connectError(err)
	}
}

func (statusInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (statusInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		return connectError(next(ctx, conn))
	}
}

func connectError(err error) error {
	if err == nil {
		return nil
	}
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		return err
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	return connect.NewError(connect.Code(st.Code()), errors.New(st.Message()))
}

package grpcserver

import (
	"errors"

	"github.com/adit-prawira/neko/internal/ffi"
	"github.com/adit-prawira/neko/internal/shared"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func HairballToGRPCStatus(err error) error {
	if err == nil {
		return nil
	}

	var hairballError *ffi.HairballError
	if !errors.As(err, &hairballError) {
		return status.Error(codes.Internal, err.Error())
	}

	code, ok := shared.HairballToGRPC[hairballError.Code]
	if !ok {
		code = codes.Internal
	}

	return status.Error(code, err.Error())
}

func StatusError(code codes.Code, message string) error {
	return status.Error(code, message)
}

const CodeInvalidArgument = codes.InvalidArgument
const CodeInternal = codes.Internal

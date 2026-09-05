package otlp

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/otelfleet/otelfleet/pkg/logutil"
	coltracespb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	// otlpcommonv1 "go.opentelemetry.io/proto/otlp/common/v1"
)

type TracesServer struct {
	coltracespb.UnsafeTraceServiceServer
}

func (s *TracesServer) Export(ctx context.Context, req *coltracespb.ExportTraceServiceRequest) (*coltracespb.ExportTraceServiceResponse, error) {
	logutil.FromContext(ctx).With("signal", "traces").Info("received")
	return &coltracespb.ExportTraceServiceResponse{}, nil
}

var _ coltracespb.TraceServiceServer = (*TracesServer)(nil)

func (s *TracesServer) handleProto(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	req := &coltracespb.ExportTraceServiceRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	resp, err := s.Export(r.Context(), req)
	if err != nil {
		// TODO : map codes
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	retData, err := proto.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}
	w.Header().Set("Content-Type", contentTypePb)
	w.WriteHeader(http.StatusOK)
	w.Write(retData)
}

func (s *TracesServer) handleJSON(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	req := &coltracespb.ExportTraceServiceRequest{}
	if err := protojson.Unmarshal(data, req); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	resp, err := s.Export(r.Context(), req)
	if err != nil {
		// TODO : map codes
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	retData, err := protojson.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(http.StatusOK)
	w.Write(retData)
}

func (s *TracesServer) handleTracePost(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")

	switch contentType {
	case contentTypePb:
		s.handleProto(w, r)
		return
	case contentTypeJSON:
		s.handleJSON(w, r)
		return
	default:
		w.WriteHeader(http.StatusUnsupportedMediaType)
		fmt.Fprintf(w, "unsupported Content-Type %s", contentType)
		return
	}
}

package handlers

import (
	"log/slog"
	"net/http"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"saybu/pkg/httpx"
)

// WriteGRPCError memetakan status gRPC ke respons HTTP standar.
func WriteGRPCError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	st, ok := status.FromError(err)
	if !ok {
		log.Error("grpc error tidak dikenal", "error", err)
		httpx.WriteError(w, r, httpx.Internal(err))
		return
	}

	var details []httpx.Detail
	for _, d := range st.Details() {
		if br, ok := d.(*errdetails.BadRequest); ok {
			for _, v := range br.GetFieldViolations() {
				details = append(details, httpx.Detail{Field: v.GetField(), Message: v.GetDescription()})
			}
		}
	}

	var out *httpx.Error
	switch st.Code() {
	case codes.InvalidArgument:
		out = httpx.BadRequest(st.Message(), details...)
	case codes.Unauthenticated:
		out = httpx.Unauthorized(st.Message())
	case codes.PermissionDenied:
		out = httpx.Forbidden(st.Message())
	case codes.NotFound:
		out = httpx.NotFound(st.Message())
	case codes.AlreadyExists, codes.Aborted:
		out = httpx.Conflict(st.Message())
	case codes.FailedPrecondition:
		out = httpx.Unprocessable(st.Message())
	case codes.ResourceExhausted:
		out = httpx.New(http.StatusTooManyRequests, "RATE_LIMITED", st.Message())
	case codes.Unavailable, codes.DeadlineExceeded:
		log.Warn("layanan downstream tidak tersedia", "code", st.Code().String(), "error", err)
		out = httpx.New(http.StatusServiceUnavailable, "UNAVAILABLE", "Layanan sedang sibuk. Coba lagi sebentar.")
	default:
		log.Error("grpc internal error", "code", st.Code().String(), "error", err)
		out = httpx.Internal(err)
	}
	httpx.WriteError(w, r, out)
}

package errors

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func RestError(ctx *gin.Context, err error, service string) {
	s, ok := status.FromError(err)
	if !ok {
		log.Printf("Unexpected error from gRPC %s server, error: %v", service, err)
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	switch s.Code() {
	case codes.NotFound:
		log.Printf("Error from gRPC %s server: %s", service, http.StatusText(http.StatusNotFound))
		_ = ctx.AbortWithError(http.StatusNotFound, err)
		return
	case codes.InvalidArgument:
		log.Printf("Error from gRPC %s server: %s", service, http.StatusText(http.StatusBadRequest))
		_ = ctx.AbortWithError(http.StatusBadRequest, err)
		return
	default:
		log.Printf("Error from gRPC %s server: %s", service, http.StatusText(http.StatusInternalServerError))
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
}

func Error(ctx *gin.Context, err error, resp interface{}) {
	if status.Code(err) == codes.NotFound {
		ctx.AbortWithStatusJSON(http.StatusNotFound, resp)
		return
	} else if status.Code(err) == codes.AlreadyExists {
		ctx.AbortWithStatusJSON(http.StatusConflict, resp)
		return
	} else if status.Code(err) == codes.Internal {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, resp)
		return
	}
}

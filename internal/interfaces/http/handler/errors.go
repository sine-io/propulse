package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	appcollection "github.com/sine-io/propulse/internal/application/collection"
)

type errorResponse struct {
	Error struct {
		Code    string                          `json:"code"`
		Message string                          `json:"message"`
		Details []appcollection.ValidationIssue `json:"details,omitempty"`
	} `json:"error"`
}

func writeError(c *gin.Context, status int, code, message string) {
	var response errorResponse
	response.Error.Code = code
	response.Error.Message = message
	c.JSON(status, response)
}

func writeValidationError(c *gin.Context, issues []appcollection.ValidationIssue) {
	var response errorResponse
	response.Error.Code = "validation_failed"
	response.Error.Message = "one or more fields are invalid"
	response.Error.Details = issues
	c.JSON(http.StatusUnprocessableEntity, response)
}

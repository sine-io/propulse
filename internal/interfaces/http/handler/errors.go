package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	appcollection "github.com/sine-io/propulse/internal/application/collection"
)

type errorBody struct {
	Code    string                          `json:"code"`
	Message string                          `json:"message"`
	Details []appcollection.ValidationIssue `json:"details,omitempty"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

type importValidationErrorResponse struct {
	Error               errorBody `json:"error"`
	AcceptedRecordCount int       `json:"acceptedRecordCount"`
	RejectedRecordCount int       `json:"rejectedRecordCount"`
}

func writeError(c *gin.Context, status int, code, message string) {
	var response errorResponse
	response.Error.Code = code
	response.Error.Message = message
	c.JSON(status, response)
}

func writeValidationError(c *gin.Context, issues []appcollection.ValidationIssue) {
	var response errorResponse
	response.Error = validationErrorBody(issues)
	c.JSON(http.StatusUnprocessableEntity, response)
}

func writeImportValidationError(c *gin.Context, issues []appcollection.ValidationIssue, rejectedRecordCount int) {
	if rejectedRecordCount < 0 {
		rejectedRecordCount = 0
	}
	c.JSON(http.StatusUnprocessableEntity, importValidationErrorResponse{
		Error:               validationErrorBody(issues),
		AcceptedRecordCount: 0,
		RejectedRecordCount: rejectedRecordCount,
	})
}

func validationErrorBody(issues []appcollection.ValidationIssue) errorBody {
	return errorBody{
		Code:    "validation_failed",
		Message: "one or more fields are invalid",
		Details: issues,
	}
}

package httpx

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// BindJSON binds and validates a JSON body, returning a 40000 AppError with a readable message.
func BindJSON(c *gin.Context, dst any) error {
	if err := c.ShouldBindWith(dst, binding.JSON); err != nil {
		return BadRequest(describeBindError(err))
	}
	return nil
}

// BindQuery binds query parameters into dst.
func BindQuery(c *gin.Context, dst any) error {
	if err := c.ShouldBindQuery(dst); err != nil {
		return BadRequest(describeBindError(err))
	}
	return nil
}

// ParamUUID parses a path parameter as UUID.
func ParamUUID(c *gin.Context, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, BadRequest(fmt.Sprintf("invalid %s: must be a uuid", name))
	}
	return id, nil
}

func describeBindError(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		parts := make([]string, 0, len(ve))
		for _, fe := range ve {
			parts = append(parts, fmt.Sprintf("%s %s", strings.ToLower(fe.Field()), ruleText(fe)))
		}
		return "validation failed: " + strings.Join(parts, "; ")
	}
	return "invalid request body: " + err.Error()
}

func ruleText(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "min":
		return "is too short (min " + fe.Param() + ")"
	case "max":
		return "is too long (max " + fe.Param() + ")"
	case "oneof":
		return "must be one of [" + fe.Param() + "]"
	case "email":
		return "must be a valid email"
	case "uuid":
		return "must be a uuid"
	case "len":
		return "must have length " + fe.Param()
	default:
		return "failed rule " + fe.Tag()
	}
}

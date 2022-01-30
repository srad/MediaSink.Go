package app

import (
	"log"
	"net/http"

	"github.com/astaxie/beego/validation"
	"github.com/gin-gonic/gin"
)

// MarkErrors logs error logs
func MarkErrors(errors []*validation.Error) {
	for _, err := range errors {
		log.Println(err.Key, err.Message)
	}

	return
}

// BindAndValid binds and validates data
func BindAndValid(c *gin.Context, form interface{}) int {
	err := c.Bind(form)
	if err != nil {
		return http.StatusBadRequest
	}

	valid := validation.Validation{}
	check, err := valid.Valid(form)
	if err != nil {
		return http.StatusInternalServerError
	}
	if !check {
		MarkErrors(valid.Errors)
		return http.StatusBadRequest
	}

	return http.StatusOK
}

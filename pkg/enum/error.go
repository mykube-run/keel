package enum

import "fmt"

var (
	ErrTenantAlreadyExists = fmt.Errorf("tenant already exists")
	ErrTaskNotFound        = fmt.Errorf("task not found")
)

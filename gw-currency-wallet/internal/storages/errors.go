package storages

import "errors"

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrDuplicateUser = errors.New("username or email already exists")
)

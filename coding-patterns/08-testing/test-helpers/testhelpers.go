package main

import (
	"fmt"
	"time"
)

type User struct {
	ID    int
	Name  string
	Email string
}

type UserService interface {
	GetUser(id int) (*User, error)
}

type RealUserService struct{}

func (s *RealUserService) GetUser(id int) (*User, error) {
	time.Sleep(500 * time.Millisecond)
	return &User{ID: id, Name: "Alice", Email: "alice@example.com"}, nil
}

func main() {
	fmt.Println("run: go test -v .")
}

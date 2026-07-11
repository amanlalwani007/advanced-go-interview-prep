package main

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type fakeUserService struct {
	users map[int]*User
	err   error
}

func (s *fakeUserService) GetUser(id int) (*User, error) {
	if s.err != nil {
		return nil, s.err
	}
	u, ok := s.users[id]
	if !ok {
		return nil, fmt.Errorf("user %d not found", id)
	}
	return u, nil
}

func TestGetUser_WithFake(t *testing.T) {
	svc := &fakeUserService{
		users: map[int]*User{
			1: {ID: 1, Name: "Test", Email: "test@example.com"},
		},
	}

	u, err := svc.GetUser(1)
	if err != nil {
		t.Fatal(err)
	}
	if u.Name != "Test" {
		t.Errorf("got %q, want %q", u.Name, "Test")
	}
}

func TestGetUser_NotFound(t *testing.T) {
	svc := &fakeUserService{users: map[int]*User{}}

	_, err := svc.GetUser(99)
	if err == nil {
		t.Fatal("expected error")
	}
}

func withTimeout(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}

func TestWithHelper(t *testing.T) {
	ctx := withTimeout(t, 100*time.Millisecond)
	select {
	case <-ctx.Done():
		t.Error("timed out")
	case <-time.After(50 * time.Millisecond):
	}
}

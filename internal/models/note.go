package models

import "time"

type Note struct {
	ID        int
	UserID    int
	Title     string
	Content   string
	Tags      string
	IsPinned  bool
	IsStarred bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

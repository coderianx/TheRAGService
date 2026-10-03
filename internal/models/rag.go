package models

import "time"

type CreateNewCollectionReq struct {
	CollectionName string `json:"name"`
	Description    string `json:"description"`
}

type Collections struct {
	CollectionID   int64     `json:"id"`
	CollectionName string    `json:"name"`
	CollectionDesc string    `json:"description"`
	CreatedAt      time.Time `json:"created_at"`
}
